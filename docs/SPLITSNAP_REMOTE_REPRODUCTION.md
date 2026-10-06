# SplitSnap 八长流：远程代码与新节点重跑指南

更新：2026-10-06。范围是 17-workload 单函数主实验，不是 IAA、BW sweep 或
十节点 trace。目标是从远程代码重新构建、采集输入并测一轮，不要求上传旧日志或二进制。

## 1. 交付边界与当前验证状态

主仓库为 `https://github.com/Liquidzk/vHive`，分支 `likun/splitsnap-sort`。
八长流运行/分析源码固定提交为
`63bd519f913741e7dfdfeb36664a50226c1a741f`；该提交的 105 个非 Markdown
运行源码/配置文件与 gate12 冻结源码逐文件一致。其后的 publication 提交补充
本指南和 rebuild 辅助材料，不宣称它们已经在新集群完成过实验。

本次只发布源码、脚本、配置模板和文档。**不上传完整实验日志、结果目录、
snapshot/WS、数据库 dump、镜像 tar、新编译二进制或 Release 附件。**
旧归档继续留在本地；新跑应现场生成这些文件。

已从 GitHub HTTPS 独立 shallow clone publication
`437fd7480304ee211ff9dad828ed5e363560434a` 和 pinned direct-invoker，未使用本地
worktree overlay：`build.sh` 的 15 个工具全部构建成功；zstdstreams、aliases、
transcode 三组 focused tests 通过；第 3 节列出的九个既有 LFS runtime/guest 文件
均实际下载成功；`build-guest.sh` 使用新生成的测试公钥完成 SquashFS 派生打包。
这只验证构建/打包，未验证新 guest 启动，也不等于新集群 103 点已重跑。
后续文档与 executable-mode 提交不更改上述受测源码。旧 runner 仍有固定节点/目录和验收材料依赖，
第 6 节列出迁移项。**目前是可获取的源码与重建路线，不是一键新节点 installer，
也不能据此声称“只 clone 后运行原 run_matrix.py 就能完成实验”。**

## 2. 需要哪些仓库与组件

| 组件 | 远程来源 / 固定版本 | 用途 |
| --- | --- | --- |
| SplitSnap relay / converter / 八流工具 / runner / 统计绘图 | `Liquidzk/vHive`, `likun/splitsnap-sort`；核心 `63bd519f913741e7dfdfeb36664a50226c1a741f` | 此次主要代码 |
| Direct invoker | `Liquidzk/vSwarm`, `snapshare-direct-invoker`, `a096cc43b84e17d57a9cea1f0a35f17a479a2db6` | `tools/direct-invoker`，直接调用 guest；不是 Knative trace loader |
| Workload 源码 | `Liquidzk/vSwarm`, `snapshare-eval-images`, `616ac7449daaa8c7eb34f8eadfe2d09292de6cd9` | benchmark、relay、MongoDB init 程序和媒体输入 |
| 四个 restore-safe 镜像的历史构建源码 | 同一 vSwarm 仓库，`f268d114706f052124f83efd43a5e70f4d810b5b` | Image-Go/Py、VideoProc、VideoAn；源码可重新构建 |
| Firecracker/containerd/shim/demux、kernel、base guest | vHive 仓库已有 `bin/` Git LFS 对象 | 现有公开依赖，不是此次新增上传 |
| Backend | MinIO `RELEASE.2024-12-18T13-15-44Z`，MongoDB、registry `2.8.3`，实验 DNS | 只在隔离私网开放；沿用配置并记录新拉取 image digest |
| Go / Zstd | Go `1.26.7`；`klauspost/compress v1.17.11`，仓库 `go.mod/go.sum` | 不执行 `go get -u` |

`invitro-fix-snapshot-caching`、根目录 PulseNet `vHive`、旧 IAA worktree 均不是本轮
direct 八流主实验的构建依赖。原 `snapshare-eval` 是非 Git 目录；此次把需要的
历史准备源码收进了 [rebuild/legacy-workspace](../experiments/currentbase17-streams8/rebuild/legacy-workspace)。

### Runtime 的重要限制

历史实测的 Firecracker-containerd/shim 带 `modified=true` build 标记，不能只根据
嵌入的 upstream commit 声称精确重建。这里使用仓库中**原已发布**的 LFS runtime。
它们与历史运行文件的内容标识一致：

| LFS 文件 | OID（SHA-256；用于识别现有版本） |
| --- | --- |
| `bin/firecracker` | `5fb7f47865abf24d55fddf1da7d96839a2a6c97396cbce64a531189329e9cb51` |
| `bin/firecracker-containerd` | `0acf74a7a337c22d8e32af06fca7a19ebf7bbc99b95923b9a5a2eb3b93739d4d` |
| `bin/containerd-shim-aws-firecracker` | `177fbb7e6035081e4201c65399ab5bb63fb192bc08da865e9e0e4c915ec73f53` |
| `bin/demux-snapshotter` | `0124f8d1c3890d4ace6ec280e3dcdbf0db69cefbfea21f1d219d96e8484d90b8` |

“从远程已有依赖重新搭建”不等于“所有底层 runtime 均能从已知未修改源码精确重编译”。
如果要求后者，需另行补齐上述 patched runtime 的完整源码 provenance。

## 3. 从远程获取、构建

在新的 x86-64 Linux 构建目录运行。准备 Git、Git LFS、Go、C 编译器、pkg-config
以及仓库 Go/CGO 依赖。SSH URL 可换为同仓库 HTTPS URL，不需要旧 aws.pem。

```bash
rebuild_workspace=$(mktemp -d /tmp/splitsnap-source.XXXXXX)
cd "$rebuild_workspace"
GIT_LFS_SKIP_SMUDGE=1 git clone --branch likun/splitsnap-sort https://github.com/Liquidzk/vHive.git vhive
GIT_LFS_SKIP_SMUDGE=1 git clone --branch snapshare-direct-invoker https://github.com/Liquidzk/vSwarm.git direct-invoker-src
git -C direct-invoker-src checkout --detach a096cc43b84e17d57a9cea1f0a35f17a479a2db6
GIT_LFS_SKIP_SMUDGE=1 git clone --branch snapshare-eval-images https://github.com/Liquidzk/vSwarm.git workloads
git -C workloads checkout --detach 616ac7449daaa8c7eb34f8eadfe2d09292de6cd9
git -C vhive rev-parse HEAD
bash vhive/experiments/currentbase17-streams8/rebuild/build.sh \
  "$rebuild_workspace/build" "$rebuild_workspace/direct-invoker-src"
```

记录 clone 到的 publication HEAD，后续同一轮固定它；不要每台节点各自拉最新分支。
`build.sh` 构建 15 个本轮工具（含 invoker），不安装服务、不联系实验节点，输出目录
必须是新目录。重新编译的文件用于新结果版本，不重标成旧 gate12 binary。

按需拉取已存在的 runtime，避免下载仓库所有历史大文件：

```bash
git -C "$rebuild_workspace/vhive" lfs pull \
  --include='bin/firecracker,bin/firecracker-containerd,bin/containerd-shim-aws-firecracker,bin/demux-snapshotter,bin/firecracker-ctr,bin/http-address-resolver,bin/jailer,bin/vmlinux-5.10.186,bin/default-rootfs.img' \
  --exclude=''
```

`bin/id_rsa` 已从新分支 HEAD 的跟踪列表移除；不复用仓库历史 guest 私钥。用户自行
生成/选择一对实验 SSH key，只把公钥装入新 guest，私钥不提交。本次不改写旧 Git 历史。

## 4. 新建运行环境和输入

### 节点与服务

三个角色：worker（Firecracker/containerd/demux/relay）、backend（MinIO、MongoDB、
registry、DNS）、loader（direct-invoker）。主实验用 direct path，不必搭完整 Knative
控制面。旧的 er020/er069/er032 仅是历史身份，不能作为新 SSH 目标。

历史平台是 c6620 / Xeon Gold 5512U，28 物理核，SMT/Turbo off，2.1 GHz；正式窗口
worker ingress IFB 为 10 Gbit/s、4 MB burst、400 ms queue limit，backend 不再叠加
TBF。400 ms 是队列上限，不是人为添加的固定 RTT。新机器若硬件不同，须修改平台
检查与报告标签，不能删除检查后继续标为 c6620。

安装配置参考仓库 `configs/firecracker-containerd/`、`configs/demux-snapshotter/`
和 rebuild 中历史 bootstrap；所有磁盘/devmapper/网络操作只可面向确认的新专用资源。
不要对借到的机器直接运行旧 bootstrap 的整套清理逻辑。

### Guest 与分类来源

`bin/default-rootfs.img` 是 SquashFS。历史可用派生 guest 比基础镜像多了
`containerd-stargz-grpc` 的实验 HTTP registry resolver。辅助脚本重做该改动，同时
安装操作者的公钥；保留所有者、设备节点和权限，不要用非 root 的不完整解包重打包。

```bash
fakeroot bash vhive/experiments/currentbase17-streams8/rebuild/build-guest.sh \
  vhive/bin/default-rootfs.img "$rebuild_workspace/guest-new.img" /absolute/path/to/experiment_key.pub
```

该脚本需要 squashfs-tools、fakeroot、ripgrep、OpenSSH 工具。保留输出的 extracted
tree，使用同一文件系统导出 `rootfs.tar` 供分类；它必须对应实际启动的 guest。
worker 使用 guest kernel 和 guest image 的路径见
`configs/firecracker-containerd/firecracker-runtime.json`，安装时按新路径更新。

每个 workload 的 `images/<image_inventory>/container.tar` 必须是该次实际 function
镜像的**展开 rootfs 文件 tar**，如从新建但不启动的 Docker 容器 `docker export`。
不是 `docker save`/OCI 外层 archive；分类程序逐个 tar 普通文件切页匹配，不会递归
打开 Docker layers。先固定 image digest，再生成分类来源和 snapshot，不能中途换镜像。

### Function images、请求和 MongoDB

17 项 workload 和请求以
`rebuild/legacy-workspace/snapshare-eval/configs/figure9_16/workloads.direct-all-r7.json`
及 `direct_requests.json` 为模板。四个外部数据库 workload 的重建脚本是
`rebuild_function_images.sh`；其余 benchmark 使用 vSwarm 内对应目录/Dockerfile。
旧四镜像 tag 是 `restore-reconnect-f268d11-r2-20260818`；新构建应使用新 tag/digest。
旧 GHCR package 可能需登录，不能把它当作唯一来源。

MongoDB 内容可以由远程 vSwarm 的媒体输入重新 seed，无需上传旧 dump：

- `benchmarks/image-rotate/images/`，请求 `img11.jpg`；对应 `init/init-database.go`，
  从该 benchmark 目录执行，参数 `--db_addr mongodb://<backend>:<port>`，生成 `image_db`。
- `benchmarks/video-processing/videos/` 与 `benchmarks/video-analytics-standalone/videos/`，
  请求 `video2.mp4`；对应 init 程序生成 `video_db`。按各服务的 MongoDB endpoint seed；
  如果共用数据库，不要重复导入同名文件造成不确定选择。

媒体如为 Git LFS pointer，先在 workloads checkout 拉取相应目录。镜像构建中的部分
上游 base tag/pip 依赖未完全固定，因此这条路线是重跑实验，不承诺与旧镜像逐字节相同。
新轮必须记录实际 digest，并在整轮实验中冻结。

### 重新生成 raw corpus，再派生六系统

1. 按 512/2048/3072 MiB 三档分别生成当前兼容的 base；workload 数为 13/3/1。
2. 17 个 workload 全部走 `regenerate-direct`，采用五次 restore-union 采 WS。
   `prepare_figure9_16_current_base_tiers.sh` 是原协议的实现入口；先完成第 6 节迁移。
3. 对同一份冻结 snapshot/raw WS/PFN 使用 `prepare_figure9_16_current_base_tiered_systems.sh`
   派生 full128K、full4K、no-image、partial、canonical full-dedup 及 oracle view。
   不为每个 baseline 各自重录一份 WS。
4. 用本轮生成的 workloads/matrix/corpora/request manifest 调用 `corpus_plan.py`，
   显式指定新 `--run-id`、`--backend`、三档 `--port-bases`、`--data-root`。
5. `inventory/` 和 `capacity/` 先检查新来源与空间；`prepare.py` 负责新的 18 stores
   copy/transcode；`finish-views.py` 完成 oracle view；随后 actual-layout、aliases、
   `local_aliases.py`、`footprint/`、`payload/` 生成本轮运行/统计材料。
   各工具 CLI 以对应 `--help`/Go `-h` 为准；不要将旧配置中的 snapshot 名直接套到新 corpus。

这些准备阶段不与正式测量重叠。旧 `config/20260910-r1` 只作为结构/语义参考，
不是新节点已经拥有这些 MinIO objects 的证明。

## 5. 八长流路径与正式矩阵

每个 coalesced payload 最多八条独立长 Zstd 流，每条用一个长 MinIO Range GET 和
一个 C1 decoder，解压到不相交的 host mmap 区间；并非不断请求 1 MiB 小块，也不是
旧的多小帧聚合。保留 CRC/每流 SHA、PFN 顺序和既有 UFFD 插页路径。

关键参数：`-wsCoalescing -wsCompression -zstdWSLayout=streams8-v1 -zstdFetchers=8`。
`-j=16` 是 UFFD pre-insertion 并行度，不是 16 个长流 decoder。

| 系统 | 路径 |
| --- | --- |
| Chunks / Pages | 原生 128 KiB chunks / 4 KiB pages GET + DecodeAll，不走八长流 |
| Sabre | full security，完整 WS 八长流 |
| SplitSnap- | no-image-sharing，private WS 八长流 |
| SplitSnap | partial，private WS 八长流；共享来源路径保持 |
| Full Dedup | 预先物化 partial-format transfer view，八长流；容量是 canonical global union 的 unsafe oracle |
| AES all-local | 与 SplitSnap remote 相同输入，storage disabled、零远端读取，仍有解压 |

正式范围：17×6 remote + matched AES all-local = 103 点。每点 60 calls，前 30
warmup，后 30 measured，absolute 1 RPS（允许重叠），不能用“等待上一调用结束”替换。
`run_matrix.py` 负责单点身份、发窗、收集与恢复；未知状态只补观察/收集，不能直接重发。
聚合入口 `aggregate.py` 生成 Figure9–13 和 CSV。指标定义见 [METHODS.md](../experiments/currentbase17-streams8/METHODS.md)。

## 6. 尚需在新节点落地的迁移项（不可跳过）

此次保留测量源码，没有为不存在的新集群猜测 IP、磁盘或伪造验收材料。
开始重跑前，须在 publication HEAD 的新运行提交中完成以下工作：

| 位置 | 需要迁移/生成 |
| --- | --- |
| 历史 shell helpers | 新 `NODES_ENV`、授权角色、`EXPERIMENT_ID`、SSH 用户、mount、binary/worktree 路径；旧默认 env 不发布也不沿用 |
| `prepare_figure9_16_inputs.sh` | all-regenerate/current-base 时仍无条件引用旧 ZIP/legacy manifest/rootfs-equivalence.txt；改为本轮生成的来源检查，不能要求下载旧日志或用空文件骗过检查 |
| `stage_figure9_16_direct_assets.sh` / bootstrap | 将旧本地 OCI、provenance、MongoDB dump 和 expired-node 复制改为第 4 节的新构建/seed；重做 registry/DNS/image map |
| `run_matrix.py` | `HOSTS`、固定 config/provenance/verification 路径、run-id/backend 检查、remote root、release 版本；应绑定本轮新配置，而非删除检查 |
| `point_config.py` / `platform_probe.py` | worker/cache endpoint、DNS、网卡名、images dir、CPU/网络检查绑定本轮实际平台 |
| `prepare.py` / `finish-views.py` / `localcache/` / `stopowned/` / `start-corpora.py` | 固定 `/users/Liquidz/streams8` 和 data-root ownership guard 随部署路径成套调整 |
| `aggregate.py` 与输入/平台校验 | 固定旧 run-id/config 路径绑定新 run；保留 103 点、完整样本和平台证据要求 |
| 验收/发布输入 | actual-layout、aliases、容量/payload、build identity、release、少量真实 restore/all-local/cancel 记录由本轮重新生成；不是从旧归档复制 |

**这张表是未完成的可移植性工作，不是宣称仅改 env 即可。** 尚未在新节点从零跑通
这些迁移步骤，所以目前不能保证完整 E2E 的远程依赖闭环。下一次借到节点后的优先项
是按表闭合配置/输入准备，确认 AES、小/大 WS、all-local 后再跑完整矩阵；不是继续
收集旧二进制或上传旧日志。若发现依赖缺失，应补来源代码/构建步骤，不伪造旧 provenance。

## 7. 最小保留与完成标准

远程保留：本分支、vSwarm 两个已发布分支、现有 LFS runtime，以及构建/部署说明。
新实验本地产生：输入清单、实际 image digest、节点配置、构建记录、调用日志、结果和图。
不必把它们全部推到 Git；但做统计和定位异常时必须在实验归档里保留。

“新节点重跑完成”须满足：从这些远程来源构建；新输入 17 项/三档匹配；六系统均使用
同一冻结 corpus；103 点完整成功且身份/平台/调用检查通过；aggregate 可输出五张图。
只通过 build、只看到图、或旧结果已完成均不等于这一条完成。
