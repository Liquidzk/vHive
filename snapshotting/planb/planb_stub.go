//go:build !sabre

package planb

func Available() bool { return false }

func openImpl(string, Options) (restorerImpl, error) {
	return nil, ErrUnavailable
}
