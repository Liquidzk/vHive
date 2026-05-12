#!/usr/bin/env bash
set -euo pipefail

ROOTFS="${1:-bin/default-rootfs.img}"
PORT="${2:-18080}"

if [[ -z "${FAKEROOTKEY:-}" ]] && command -v fakeroot >/dev/null 2>&1; then
	exec fakeroot -- "$0" "$@"
fi

if [[ ! -f "$ROOTFS" ]]; then
	echo "rootfs not found: $ROOTFS" >&2
	exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/vhive-rootfs-clock-sync.XXXXXX")"
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT

root="$tmpdir/rootfs"
unsquashfs -q -d "$root" "$ROOTFS" >/dev/null

install -d "$root/usr/local/bin" "$root/etc/systemd/system/firecracker.target.wants"

cat > "$root/usr/local/bin/vhive-clock-sync-handler.sh" <<'EOF'
#!/bin/sh
host_unix=""

while IFS= read -r line; do
	line="$(printf '%s' "$line" | tr -d '\r')"
	[ -z "$line" ] && break

	key="$(printf '%s' "${line%%:*}" | tr '[:upper:]' '[:lower:]')"
	value="${line#*:}"
	case "$key" in
		x-vhive-host-unix|x-khala-host-unix)
			host_unix="$(printf '%s' "$value" | tr -d '[:space:]')"
			;;
	esac
done

status="200 OK"
body="ok"
case "$host_unix" in
	""|*[!0-9]*)
		status="400 Bad Request"
		body="missing or invalid host unix time"
		;;
	*)
		if ! /usr/bin/date -u -s "@$host_unix" >/dev/null 2>&1; then
			status="500 Internal Server Error"
			body="date set failed"
		fi
		;;
esac

printf 'HTTP/1.1 %s\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: %s\r\n\r\n%s' "$status" "${#body}" "$body"
EOF
chmod 0755 "$root/usr/local/bin/vhive-clock-sync-handler.sh"

cat > "$root/etc/systemd/system/vhive-clock-sync.service" <<EOF
[Unit]
Description=vHive guest clock sync HTTP hook
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/socat TCP-LISTEN:${PORT},reuseaddr,fork SYSTEM:/usr/local/bin/vhive-clock-sync-handler.sh
Restart=always
RestartSec=0

[Install]
WantedBy=firecracker.target
EOF
chmod 0644 "$root/etc/systemd/system/vhive-clock-sync.service"

ln -sfn /etc/systemd/system/vhive-clock-sync.service "$root/etc/systemd/system/firecracker.target.wants/vhive-clock-sync.service"

new_rootfs="$tmpdir/default-rootfs.img"
mksquashfs "$root" "$new_rootfs" -noappend -comp gzip -b 131072 -all-root >/dev/null

backup="${ROOTFS}.pre-clock-sync.$(date +%Y%m%d%H%M%S)"
cp -f "$ROOTFS" "$backup"
cp -f "$new_rootfs" "$ROOTFS"

echo "updated $ROOTFS"
echo "backup: $backup"
