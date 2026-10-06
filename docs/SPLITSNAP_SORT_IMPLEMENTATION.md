# 八长 Range / 八 decoder：实施细节冻结稿

日期：2026-09-10。分支 `likun/splitsnap-sort`，基线 `3f18475f`。

**当前指令：用户 goal continuation 恢复按本文完成完整实施。**
上一轮文档-only边界已被本次实施指令覆盖；§0的设计/计时/归因约束继续有效。
本轮已核对 gate12 部署/源码身份、精确退役旧失败点所属网络，并推进同版负向与
生命周期验证；实时接续以 [STATUS](SPLITSNAP_SORT_STATUS.md) 为准。
目标仍完整17×6 + matched AES all-local、新统计/图与复现来源，不缩为五函数。
原服务/corpus/结果保留，不重标 gate11 点。2026-09-11已完成同版103点/6,180calls，
生成并核对新Figure9–13、WS质量/资源CSV及方法说明；当前交付核验见
[最终逐项审计](SPLITSNAP_SORT_FINAL_AUDIT.md)，历史过程见
[验收索引](SPLITSNAP_SORT_ACCEPTANCE.md)。下文待办/候选文字保留设计历史，
不按未打勾的旧清单重复实验，最终完成状态以最终审计及交付清单为准。

先读 §0（本次核对及真正后续事项），再读 §1–9 的设计契约。
§3 中的旧行为、§10–14 的缺口均保留历史上下文；有冲突时以 §0 为准，不能照旧
清单重复实现。详见 [系统迁移计划](SPLITSNAP_SORT_PLAN.md) 和
[候选/证据状态](SPLITSNAP_SORT_STATUS.md)。设计确认不等于完整系统验收。

## 0. 本次确认与下一次 goal 待办

### 0.1 不再悬而未决的实现细节

1. **编码先建立独立性。** 同一 raw WS 按原页序等分为最多八段，逐段各编码一条
   独立 Zstd 长流，直接顺序写到一个 payload 文件；manifest 记录两套偏移/长度。
   不把已有单压缩流的字节任意切八段。旧压缩输入若无 raw，离线还原原字节后
   重编码，不重新录制 WS。标准格式仍有八个 frame，移除的是大量应用层小帧。
2. **一个长 Range 对应一个 decoder。** MinIO 参数为 offset/length，底层实际
   HTTP 区间是 `[offset, offset+length-1]`。八个 reader 各自消费完整独立流，
   decoder concurrency=1；不反复请求 1 MiB、不共用同一 Object 并发 Seek。
   decoder 随 reader 增量输出，不等整个 payload 下载完成；每条流内部不另外
   创建下载队列/后台预取线程，不承诺单路网络与 CPU 的完全重叠。
3. **没有解压后拼接，但仍有 staging。** 八路写同一 raw mmap 的不相交区间，
   全部解码/校验完成才进入既有 UFFD 安装。不是 direct-to-guest 或零拷贝。
   clean remote 不保留完整压缩副本；非 clean remote 的 TeeReader 仍填充整份
   compressed-cache buffer，并在成功后异步发布；local 用同一 fd 的 SectionReader。
4. **保留原实验变量。** level 3、固定 klauspost 版本、8 MiB window、checksum/
   每流 SHA、原页序/PFN/分类/共享不变。SHA 是每条流完成后在该流 goroutine 中
   计算，可能与其它流下载/解码重叠；它仍在 WS wall 内，不是此前无 SHA 微基准。
5. **八路是每对象上限，不是整次恢复八个 HTTP 请求。** manifest、recipe、WS
   index、snapshot 信息和 lazy fetch 都仍存在，SDK 重试也另算。按 raw 页数等分
   不保证压缩字节/CPU 等分；慢一路、小 WS、八个 decoder 的内存成本仍需实测。
6. **迁移范围不缩小。** 四个 coalesced 系统均迁移：Sabre full WS、No-image/
   SplitSnap 各自 private WS、Full Dedup 的 partial-format oracle transfer view。
   Chunks/Pages 保留原生 128-KiB/4-KiB 路径。后续完整终点为 17×6 remote +
   matched SplitSnap AES all-local、Figure9–13 与 WS quality CSV；BW/trace 单列。

### 0.2 已有候选与证据：不要从头重写

本次只读 `zstdstreams/streams.go`、`manager_streams.go`、MinIO Range 接口、
CLI、runner/aggregate 的版本入口、已有 METHODS 和本地 release/收尾记录。
没有重跑测试/validator、查看活动 PID/tmux 或检查当前远端进度。

| 层次 | 已有入口/材料 | 后续重点 |
| --- | --- | --- |
| 编码/解码 | `snapshotting/zstdstreams/streams.go` | 核对固定版本与真实部署；不是只写 codec 微基准 |
| runtime/cache | `snapshotting/manager_streams.go`、`ctriface`、`uffd_handler` | 保留 context、成功发布、取消后等写入退出及 VM/UFFD 释放契约 |
| 六系统准备 | `experiments/currentbase17-streams8/` 中 converter/copy/view/aliases/inventory 工具 | 按身份复用已生成 raw/PFN/corpus；不由旧清单推断全部未实现 |
| gate11 局部释放 | `verification/remote-20260910-r1/aes-negative-gate11/shutdown.json` | 本地已有七个所属 VMM 缺席证明；§14 的“尚缺 shutdown”已过时，不重测两个已验收负向 |
| 统计绑定 | `accounting_inputs.py`；runner 冻结 accounting/release；正式 aggregate 读 frozen | §14 的“正式聚合仍读可变来源”已过时；后续核对选定结果与完整报告，不重扫 corpus |
| 版本归档 | `provenance/20260910-r1/gate11-release.json`、完整源码 overlay-r3、toolset-r3 | r3 包已包含 late-cancel/accounting/runner；旧初版工具包的缺项不能套到 r3 |
| 历史测试 | `verification/local-20260910-gate11/python-tests-release-r3.log` | 已有 40 tests/OK；不代表全仓测试通过，旧 ctriface API 编译失败记录仍保留 |
| gate12 候选/版本入口 | `run_matrix.py` 已默认 gate12；aggregate 接受 gate12；`provenance/20260910-r1/gate12-release.json` 指向完整源码/toolset-r1 | “尚未接入 runner/尚无 release”已过时；存在本地文件不证明已部署或同版负向/完整矩阵验收通过。本轮不执行构建、包比对或部署检查 |
| gate11 失败点的诊断收尾 | `results/20260910-r1-gate11/points/no-image-zstd3--image-rotate-go-11--remote/diagnostic-retirement.json` | 已保存 MainPID=0、service failed、65个所属VM及 remaining_processes=[]；仅诊断记录，不转正该点，不是正常 shutdown 验收，也不证明网络资源已释放 |

表中实验路径均相对 `experiments/currentbase17-streams8/`，代码路径相对仓库根。
release 描述归档时版本；本次文档修改不重写 gate11/gate12 归档或改变运行 binary。
gate11/r3 的局部验证仍只证明原版本，不能代替 gate12 的同版验证；但已有输入/
转码来源可按身份复用。历史“relay126349仍active”“gate12仅本地build”的文字
不是当前事实声明，不能用来跳过下一轮的实际状态检查。

### 0.3 用户开 goal 后的最短接续顺序

- [ ] 先核实具体工作树与既有任务身份。历史入口为
  `results/20260910-r1-gate11/controller-matrix-r1/`、
  `streams8_formal_gate11_matrix_r1`，以及 gate12 release/部署材料。本轮未检查活动
  任务、端口、网络或节点状态。先区分已结束窗口、仅待补收材料与新版本所需窗口；
  不因旧完成数、缺本地文件或超时重启。若已有任务，先确认其与新 goal 的关系。
- [ ] 对照候选/归档/部署版本和已有 gate，只修真实缺口；固定输入、CPU/SMT/10G
  与隔离资源。保留旧服务；准备/统计 I/O 不与测量重叠，不重复五函数冒烟/全库 SHA。
- [ ] 若采用 gate12，先核对该版本的必需 WS 缺失、真实 LoadSnapshot 后取消及
  整个所属 VM 池退出材料；不能借用 gate11 的通过记录。诊断收尾没有清理网络/
  cache；获准后先查精确所有权与占用，再决定是否需要针对性收尾，不按 `sstr*`
  通配符批量删除，也不把清理动作当作性能点验收。
- [ ] 验收完整 103 点：每点 60 calls，前 30 warmup、后 30 measured，保留失败/
  退化/长尾。窗口完成还要有收集、VM/UFFD/整个所属池退出及资源记录；同一窗口
  收集中断只补收。新 binary 不重标旧样本，旧 INVALID 不转正。
- [ ] 使用 frozen 静态输入和同版测量生成新 Figure9–13、WS quality CSV、
  指标公式及来源/失败说明；不覆盖旧图。Figure13 仍排除 recipe，运行时不删除它。
  最后逐项核对源码/构建/工具、102 行输入、103 点证据与新图，才算全系统完成。
- [ ] 分开写“新系统结果”和“八流布局的独立收益”：候选还含缓存/错误处理/生命周期
  修正，不能将它与旧二进制的全部 E2E 差异归因于八流。若要给布局单独归因，另列
  同输入、同平台、同校验/计时及其余代码匹配的对照；不为本轮文档任务追加实验。

本轮不执行这些待办，也不把已有局部 evidence 或历史“已启动”当作完整交付。

### 0.3.1 开发与性能验收分开

实现方向已确定，不再等待“1-MiB循环请求还是长 Range”的选择。下次 goal 先补齐
真正缺少的系统集成与版本证据，再按既定矩阵收数；不能反过来用单点性能好来
跳过正确性。允许结果显示小 WS 无收益、压缩率或 E2E 退化，必须原样报告。
完整103点是同版新系统验收，不等于自动授权额外的旧版全矩阵 A/B、BW sweep、
十节点 trace、IAA 迁移、metadata 并行化或去掉 SHA。

### 0.4 本次细节复核：实际接口与可作出的结论

以下按当前 dirty 候选静态核对，没有运行 benchmark/validator 或查询活动任务。

| 本地源码位置 | 已核对的细节 | 后续验收/表述限制 |
| --- | --- | --- |
| `zstdstreams/streams.go::EncodeTo` | 每段独立 NewWriter/Write/Close，直接顺序写同一 writer；页数均分，显式 C1/CRC/8-MiB window | 一个大 payload 仍含最多八个标准 Zstd frame；“不用分帧”指不再产生大量 1-MiB 应用层小帧，不是无 frame 格式 |
| 同文件 `decode`；`storage/minio_storage.go::OpenObjectRange` | 每段一次 open，HTTP inclusive end=`offset+length-1`；每条 goroutine 持有自己的 reader/C1 decoder，直接读入自己的 raw extent | 八路并行是每个 payload 的上限；不是循环 1-MiB GET，也不是 SDK 的实际 HTTP attempts 计数 |
| `manager_streams.go::getWorkingSetPathManagedContext` | remote manifest 先到，随后 mmap/八路 decode；local 是一个 fd 的八个 SectionReader；非 clean 仍 Tee 到压缩缓存 | 小 manifest 的 ReadAll 合法；payload 不走整包 DownloadObject。直接写输出区间不等于零拷贝/不使用 staging，也不保证单路网络和 CPU 完全重叠 |
| `ctriface/iface.go::LoadSnapshot` | 当前候选先取 UFFD memory/recipe，再取 WS 页号，随后取 WS sources/content；这三段在此调用层串行，之后才启动 UFFD handler | 八流只覆盖 coalesced payload；不是把 recipe/index/shared 准备一并改成八路。首版不夹带 metadata 并行化；若之后改它，必须重新核对组件计时边界 |
| `manager_streams.go`、`zstdstreams/streams.go::StreamStats` | `FirstReadUS/ReadDoneUS` 来自应用 reader 返回字节，`FirstOutputUS` 来自 decoder.Read 返回输出，`VerifyUS` 是各流 SHA wall | 是应用层流式证据，不是 socket 到包时间/链路利用率。小流可能已读完才有输出，不据此认定实现退回整包下载 |
| `ZSTD_WS_DECODE` / `DecodeStats.ElapsedUS` | 前者覆盖 manifest、mmap、GET/解码/校验及同步记账；后者是八流 join 的嵌套 wall，异步磁盘发布不在等待边界内 | 不把两者相加，也不相减得到“纯网络时间”；八个 VerifyUS 之和不能直接从 outer wall 扣掉得到无 SHA 性能 |
| relay/converter CLI；`manager.go::ConfigureCompression` | WS compression 默认 streams8-v1，fetchers=8，旧 FrameSize 非零被拒绝；六系统启动参数需要与同版候选对应 | `-j16` 仍是原恢复/UFFD 并发，不是 decoder 数；不能仅改启动标签就算迁移 |
| `accounting_inputs.py`、`run_matrix.py`、`aggregate.py` | runner 已冻结静态 reports/release；正式 aggregate 读 frozen，103 点还核对 shutdown/resources | 不照历史“统计未冻结”条目重写；POINT_COMPLETE 文件早于整 unit 收尾，不单靠它作为完整发布证明 |

首版按原 raw 页数均分，不引入动态重分片或排序；压缩字节数、解码耗时未必均衡，
完成时间受最慢流制约。预期收益来自减少小帧 GET/decoder 调度和较长压缩历史，
代价包括八路解码内存、负载不均与单路流式背压；**不预先承诺一定更快或 8× 加速**。
若要归因于布局，后续保持平台、输入、校验和计时一致做 matched 比较，而不是用
无 SHA 的独立 codec 实验代替系统结果；BW/trace 不自动扩入本轮范围。

已有外部依赖记录也不应遗漏：`provenance/20260910-r1/` 下的
`external-direct-invoker.json`、`external-worker-runtime.json`、配置归档及
`aggregation-environment.json` 不属于 relay 源码 overlay 本身。worker 的两个
构建记录为 `modified=true`，仅有嵌入 commit 不足以重建，后续保留已匹配的 binary；
记录中的 rootfs/kernel 路径不等于当前可用性证明。本轮只读已有记录，没有新增
二进制校验、远端配置采集或版本检查。下一 goal 首先按 §0.3 核实同一任务和依赖，
再决定接收既有结果或补真实缺口，不从头重复启动实验。

### 0.5 本次确认后的范围与后续 goal 描述

当前没有需要用户再选择的布局参数：固定最多八条长流、八长 Range、八个 C1
decoder，沿用原页序和 Zstd-3/校验设置。剩下的是后续版本/输入/实测验收，
不是再比较“每次取1 MiB”与“八长 Range”两种设计。

旧 `zstdstream/frames.go` 也直接写 destination，并已有增量 Range 读取；因此
本次不能描述为首次实现流式或“消除原有最终拼接”。准确变化是：从随 raw 大小
增长的小帧/GET/decoder 任务，改成固定上限八条独立长流；写入时仍生成标准 Zstd
frame，恢复时仍保留 host staging 和既有 UFFD 安装。旧对象不能只改后缀复用，
raw/PFN 内容可以复用，新的压缩 payload/manifest 与对象大小统计必须匹配。

Figure11 中 SplitSnap 的蓝段仍按 `download + get_uffd + get_ws_pages +
get_ws_content` 统计，不等于 private WS GET；WS content 内还嵌套 manifest、
mmap、流式读取/解码/校验等开销。具体公式见
[METHODS.md](../experiments/currentbase17-streams8/METHODS.md)。保持 SHA，不从嵌套
wall 相减伪造纯网络/纯解压时间；也不将之前无 SHA 的五函数微基准替代系统测量。

后续可以直接以此开 goal（本轮不执行）：

> 按本文有效设计完成 `likun/splitsnap-sort` 的系统级八长 Range / 八 decoder
> 替换。先核对已有候选、同一历史任务与归档，复用有效输入及结果，只补真实缺口；
> 覆盖 converter/runtime/cache、四种 coalesced 路径及两种 native baseline，
> 验收17×6 remote + matched SplitSnap AES-Go all-local（每点60次、30+30），
> 输出新 Figure9–13、WS quality、方法与复现来源。保留原服务/结果/图，BW与
> C3/r12 trace另列，不以局部 codec 或 AES 点代替完整验收。

### 0.6 开始实现前的检查卡（本轮静态复核）

本轮再次核对 `zstdstreams/streams.go`、`manager_streams.go`、
`storage/minio_storage.go::OpenObjectRange` 和旧 `zstdstream/frames.go`；
结论是保留上述八长流方案，不新增下载策略或实验范围。接手时重点防止以下误读：

- **对象发布有先后。** 候选先上传完整 payload，最后上传 manifest；本地缓存也
  以完整 payload/manifest 配对命中。这个约定依赖 revision 对象不可变，不能在
  原 prefix 上覆写 payload，却让旧 manifest 或旧缓存继续可见。新表示用新命名空间，
  冷别名在对象齐备后发布；已有同身份输入按记录复用。
- **reader 打开不等于 HTTP 已完成。** Range 包装返回 MinIO reader，实际读取由
  decoder 消费推动；`RangeOpens` 是应用层打开次数，不是实际 HTTP attempts。
  不加应用层循环 1-MiB 请求或重试队列；失败交回现有调用/记录路径，不能偷偷回退
  整包下载。SDK 行为和服务端带宽不能由“八个 reader”推断已验收。
- **并发与内存按对象计。** k 个重叠的 coalesced restore 最多有 8k 个流任务，
  不是全节点只有八个 decoder。每对象仍分配完整 raw mmap，非 clean miss 另有
  完整 compressed-cache buffer；还有 decoder/SDK 内部缓冲。不能把八个 8-MiB
  window 当总 RSS 上限，也不在首版额外引入全节点调度器。
- **收益只是待测假设。** 大 WS 可减少 GET/decoder 创建与任务调度，并延长压缩
  历史；页数均分不保证压缩大小或解码时间均衡，完成仍等最慢流。主实验保留原
  cadence，不能把重叠窗口当“孤立 fetch”，也不能保证所有 workload 都加速。
- **先辨认既有工作，再补缺口。** 已有候选和历史 32/103 等状态不等于本轮实现
  或实时观测。获准后先核对同一 controller、结果及版本；不按旧缺项从头重写，
  不因本文待办未打勾就重复测量。完整验收顺序见 §0.3。

## 1. 已确定的实现选择

| 项目 | 首版约定 |
| --- | --- |
| 压缩布局 | 一个 payload，最多八条独立长流；不再按固定 1 MiB raw 分帧 |
| 下载 | 每条流一个完整压缩区间 Range GET，增量读取；不循环请求 1 MiB ranges |
| 解压 | 每条流一个持续存在的 decoder，内部 concurrency=1；每对象最多八路 |
| 输出 | 直接写同一个 raw mmap 的不相交区间；八路完成后交给现有 UFFD |
| 缓存 | 保存压缩表示；local SectionReader 解压；不改成缓存解压后的 WS |
| 范围 | 17×6 配置及 AES-Go all-local；Sabre、No-image、SplitSnap、Full Dedup coalesced view 都迁移 |
| 不改变 | 页序、WS/PFN、分类/隔离、shared pinning、Chunks/Pages native granularity、UFFD 安装算法 |

“八个 Range”仅指一个 coalesced payload 的正常读取，不是整次 restore 总共八次
请求：manifest、recipe、snapshot metadata、WS index、lazy pages 仍然存在。
一个对象仍是一个对象，不能把 `object_count=1`、`stream_count=8` 与 GET 数混为一谈。
首版不做 1 MiB 预取队列、64 路下载、decoder 池、按压缩大小动态重新分片或内容排序。

## 2. 格式与接口约定（设计契约，已有候选代码）

统一名称：CLI/layout 使用 `streams8-v1`；manifest 的 format 使用
`zstd-streams-v1`。两者不是两种布局，前者表示固定八路的运行配置。

- 新包：`snapshotting/zstdstreams`，不把旧 `zstdstream.Frame` 改名后继续逐帧调度。
- Payload：`<raw basename>.zstd.streams`。
- Manifest：`<raw basename>.zstd.streams.json`。注意旧 manifest 实际是
  `.zstd.json`，**不是** `.zstd.frames.json`。
- Manifest 字段：`version=1`、`format`、`codec=zstd`、`level=3`、
  `page_size=4096`、`stream_count`、`raw_size`、`compressed_size`、`streams[]`。
- 每条 stream：`index`、`raw_offset/raw_size`、`compressed_offset/compressed_size`、
  `raw_sha256`。压缩/原始偏移各自连续，extent 必须匹配总大小；格式错直接报错。
- 不增加旧 `frame_size` 字段，也不以它控制流数。

设原始页数 P，N=min(8,P)，前 P%N 条各取 P/N 向下取整加一页，其余各取
P/N 向下取整页；P=0 时 N=0。输入非 4-KiB 整页直接报错，不补零。
正式 17 份 private WS 都超过八页，所以均为八条；小/空边界仅用于正确性测试。

建议核心接口：

```go
EncodeTo(dst io.Writer, raw []byte, level int) (*Manifest, error)
type OpenRange func(ctx context.Context, offset, length int64) (io.ReadCloser, error)
Decode(ctx context.Context, m *Manifest, open OpenRange, dst []byte) error
```

EncodeTo 逐段创建 writer、增量 Write、最后 Close，直接写文件并累计压缩偏移；
不先生成八份完整压缩 slice 再拼接。离线只统计压缩大小时也用同一编码器，输出到
计数字节数的 writer，不保留另一套 `Encode(..., 1MiB, 3)` 算法。
Decode 每条流一个 goroutine，不再有按小帧循环取任务的 worker queue。

固定现有 klauspost/compress **v1.17.11**：level 3 对应 `SpeedDefault`，该版本
默认编码窗口为 8 MiB；首版保持这一设置、CRC 开启、encoder concurrency=1。
解码 concurrency=1，显式接受最多 8 MiB window，并保留该版本其它默认行为。
窗口上限不是 decoder 总 RSS 上限，八路内存不能仅按 8×8 MiB 宣称已封顶。
依据本机固定版本 `zstd/encoder_options.go`、`decoder_options.go`，不是未核实的新版本。

新 runtime `-wsCompression` 开启时默认使用 `streams8-v1`，runner 仍显式记录
`-zstdWSLayout=streams8-v1`。八路是该模式契约，不能配置 W10 然后实际悄悄跑 D8。
现有 `CompressionConfig.FrameSize/Fetchers`、CLI 和 helper 默认 W10 均需同步清理：
新 WS 路径不接受旧 frame-size 参数；若保留 fetchers 参数，只接受该模式的 8。
Chunks/Pages 自身压缩仍由 `chunkCompression` 控制，不受 WS layout 约束。
raw/source-learning 路径保持原语义。旧 frame codec 仅供显式离线转码/旧版本对照，
不为新 runtime 添加按文件存在性自动回退。

## 3. 本次代码审计发现：不能只换编码器

下列位置均以分支基线为准，实施后行号会变化。

| 已核实位置 | 当前行为与必要处理 |
| --- | --- |
| `snapshotting/zstdstream/frames.go::Decode` | 内部建取消 context，但 OpenRange 不带 context；已开始的阻塞 Read 不会仅因 worker cancel 自动结束。新接口传递同一个派生 context。 |
| `snapshotting/manager.go::getWorkingSetPathManaged` | Decode 和 MinIO OpenObjectRange 都传 `context.Background()`；改为来自真实 LoadSnapshot 的 context。 |
| 同上 | 非 RangeObjectStorage 会整对象 DownloadObject，再分 reader；新远端路径必须要求 RangeObjectStorage，不保留这个隐式整包 fallback。 |
| 同上 | 非 clean 模式先分配整份 compressedCache，用 TeeReader 填不同区间；这不是解压前的下载屏障，可以首版保留并计入内存。 |
| 同上 | 成功后异步 `os.WriteFile` 写最终 payload，读取端以 Stat 判命中；需要临时文件+完整发布，不能让新读者打开尚未写完的文件。 |
| `GetWorkingSetContentManaged` | private 出错会尝试 full WS；新 compressed restore 按 policy/view 选定输入，损坏/缺失不能被另一对象掩盖。 |
| `GetWorkingSetContentSourcesManaged` | private index 读取失败目前会变 nil；新正式 compressed/coalesced restore 需要明确失败，而不是退回 lazy path 后仍算成功。 |
| `ctriface/iface.go::LoadSnapshot` | 当前 WS sources/content 错误仅 Warn，可能继续 UFFD；新正式路径需把必需 WS 错误返回调用方，释放已取得的 mem/WS buffers。 |
| `storage/minio_storage.go::remoteFetchClass` | 只认 `.zstd.frames/.zstd.json`；增加新 payload/manifest 分类，不能计到 other。 |
| `WorkingSetRegistry` / `workingSetCacheFileNames` | 新文件需参与 byte charging、eviction、启动恢复；仅修改读取后缀会漏计。 |

上述严格失败仅针对 **已冻结 WS、非 recording、需要 compressed/coalesced WS** 的
正式恢复；base/source/首次 learning 合法无 WS 的情况保持不变，不做全局错误策略重写。
Full Dedup 的对象选择以 formal transfer-view 为准，不能简单因 security 字符串就
切到另一种 monolithic WS。缺失和损坏输入不能静默变成“纯按需取页”的有效样本。

### 取消与释放顺序

`LoadSnapshot(ctx)` → 新 Managed WS context 接口 → Decode 派生 context →
每个 MinIO `OpenObjectRange(ctx, ...)`。包括 manifest 获取也应使用可取消 reader，
不再让新 WS 入口调用内部固定 Background 的 DownloadObject。
小 manifest 可以完整读取；禁止整包读取的是 payload。

任一路出错：保存原始错误 → cancel 其它请求 → 关闭各 reader → 等所有 decoder/
写入任务退出 → 释放失败输出 mmap。成功输出继续由 UFFD 生命周期的 release 接管。
本地 read/decoder 也检查取消；不要让 Close/取消回调与普通 defer 重复释放底层资源。
不能给整个恢复另塞一个任意短 timeout；沿用调用方 deadline。

### 缓存发布与淘汰的最小修改边界

保留现有“恢复成功后异步落 compressed cache”的时机，避免顺便把磁盘写入变成
新的同步延迟。只做以下必要修正，不重写缓存算法：

1. 网络/解压全部成功才允许发布；临时文件不进 inventory，也不能成为命中。
2. payload 写同目录独立临时文件，关闭后 rename；manifest 是对应完整对象的提交
   信息，不经通用 `getFileContent` 提前异步发布。只有完整匹配的一对才可缓存命中。
3. 同 revision 发布、命中检查/打开与 eviction 使用一致生命周期保护；异步任务
   携带 generation/token。eviction/清理使旧 token 失效，过期发布丢弃，避免淘汰后复活。
4. 复用现有 `WorkingSetRegistry.deletionLock` 时固定锁序：deletion → stats；
   不在持有 statsLock 时反向获取 deletionLock，不持全局锁等待网络或解压。
5. 本地命中在受保护区间打开文件，之后可用同一 fd 的八个 SectionReader/ReadAt；
   已打开文件允许 unlink 后继续读，全部 decoder 结束再关 fd。避免八次 Stat/Open
   跨越 eviction 导致“检查时存在、读取时丢失”。
6. 不额外引入每次 restore 的 fsync 或压缩缓存重拷贝；一次完整成功发布登记一次
   access。异步保存失败不把已成功解码改成应用失败，但必须记录 cache publish 失败。

不要求把所有旧 metadata 缓存机制重构；只确保新 payload/manifest 及其生命周期
一致。并发读/发布/淘汰测试验证这一契约，不能只用 rename 就宣称完整 race 已解决。

## 4. 全流程容易漏掉的工具依赖

- Full Dedup：`cmd/materialize_full_dedup_oracle_ws` 位于另一个 dirty worktree。
  除真正 encode 路径，还有 SplitSnap-view validation、batch/global-union 统计和
  报告中写死的 `ZstdFrameSize=1048576`、privateFrames、对象后缀，均需适配。
  当前正式 view 复用 partial private/shared 表示；不能改成 monolithic canonical WS
  冒充相同 baseline。只移植必要文件，不 merge 原 worktree 全部未提交修改。
- `prepare_figure9_16_full_dedup_oracle.sh::write_copy_plan` 写死四个 private 文件，
  包括 raw、旧 payload/manifest、private index；新版按实际新对象清单生成，不继续
  依赖旧的固定“每 revision 四项”计数。
- `prepare_figure9_16_cold_aliases_direct.sh` 不是旧后缀白名单：它复制除 mem_file、
  snap_file 之外的所有对象，再最后复制 snap_file 作完成标记。因此要先保证新 corpus
  不混旧 frames；新 alias 只验 snap_file 存在不够，还需对 layout inventory。
- `analysis/figure9_10_currentbase17.go` 当前读完整 WS frames，并将共享/global union
  用旧 1 MiB Encode 重新估算；全部改用同一新编码器。分母、去重身份、首次出现顺序
  不变；native snapshot chunk 编码不改。
- `collect_figure13_paper_payload.py` 当前按后缀取 payload 大小；新 collector 从
  layout inventory 选择文件。图继续取 `compressed_payload_bytes`，不取包含 recipe
  的 `logical_restore_bytes`。不能用八个 range 数当作八个存储对象。
- `plot_figure11_aes_exact_counter.py` 明确旧 `ZSTD_WS_DECODE` 包含 manifest/mmap/
  GET/解压，是嵌套 wall timer。新 marker 保留这一边界，增加 layout/streams 字段；
  不把这个值再加到 `GetWorkingSetContent` 上，也不以相减拆出独立网络耗时。
- runner 的 binary、helper、corpus、aliases、result 路径均有历史默认值，部分
  节点/授权 guard 写死。派生版本必须显式带新目录与配置，不只改一个 relay 路径。
  原 guard 不绕过；未来核实实际节点后，建立对应的新实验配置。

新代码/脚本统一归入本分支的 `experiments/currentbase17-streams8/`；需要保留的
外部依赖列清单。历史 snapshare-eval 脚本、输入、MinIO prefix、图和 COMPLETE 不覆盖。
生成完整 102 行 inventory，至少记录 system/profile/tier、raw/index 来源、layout、
payload/manifest key、raw/compressed bytes、stream count、binary commit/配置。
原 Full Dedup/partial 共享 transfer view 如字节相同应如实报告，不人为制造差异。

## 5. 最少但有效的验证与计时

先做一组 focused tests，而不是反复跑五函数 codec 冒烟：

- roundtrip、页边界、空/不足八页、乱序完成，输出精确一致。
- 延迟 reader 阻止输入尾部到达，通过同步观测确认先有解压输出；不能靠运行时
  猜测或未同步读取 destination 造成 data race。小于一个 Zstd block 的极小流不保证
  提前产生输出，用足够大的多 block 输入验证。
- 截断/损坏一路、其它 reader 阻塞：取消能退出，所有写任务先于释放结束。
- 顶层 compressed WS 缺失确实令 restore 报错，而不只是底层 codec test 报错。
- remote 成功发布/local 命中、并发发布/淘汰，失败不留下可命中的半文件；all-local
  远端请求和读取字节均为 0。保留 Chunks/Pages native 回归。

随后才在独立命名空间进行小/大 WS 各一个真实 MinIO→UFFD 检查，通过就进入完整
17×6 + AES all-local。保留已有每流 raw SHA、CRC/长度/EOF，不额外增加多轮全库
SHA 扫描；也不在此次布局迁移顺带删除 SHA，从而混淆性能变化来源。

记录 outer WS wall、manifest/mmap、每流读入/首输出/完成、校验耗时及 bytes。
每流 decode wall 含等网络，不叫纯解压 CPU time；并行耗时不相加当作 critical path。
当前 MinIO `countingReadCloser.requests` 是首次成功读到字节计一次，不是 HTTP
请求审计（看不到失败的零字节请求/SDK 内部重试）。新日志分别记录 range opens、
成功 streams、应用读取 bytes；八条成功流不据此承诺线上的 HTTP attempts 必为八。

## 6. 下一次 goal 的入口与未验证事项

本次本地代码审计足以确定设计，**不代表环境 ready**：本轮未重新确认节点存续、SSH、
三档全部 raw/index 可读取、剩余磁盘、CPU/限速或是否有其它活动实验。
实施 goal 先安全核对这些状态；沿用活跃服务，不停止旧 relay/Firecracker/数据库。
如果新部署需要接管已有服务而无法隔离，先报告所需权限，不能以“完整替换”为由
直接清空旧 cache/corpus 或停止无关实验。

下一次可直接以如下内容作为 goal 的核心描述：

> 按 SPLITSNAP_SORT_PLAN.md 和 SPLITSNAP_SORT_IMPLEMENTATION.md，在
> likun/splitsnap-sort 上完成 17-workload 主系统八长 Range/八 decoder 迁移，
> 包括 converter、全部 coalesced policies、Full Dedup view、缓存、runner 和统计。
> 核实环境后复用身份一致的独立新 corpus，仅补缺失准备，完成 17×6 + AES-Go
> all-local，保留退化和失败记录，
> 新结果/图另存，不覆盖旧分支/输入/结果。不能以五函数 codec 小测试代替系统完成。

带宽 1/2/5/10G 和十节点 trace 的依赖保留在主计划 P4；是否纳入下一次 goal 的
执行终点，以用户届时给出的范围为准。17-workload 验证完成不能代表 r12 scope-cache
trace 已完成迁移。若旧帧和新流要作性能归因，需同环境小规模 matched A/B；不因
新布局必须更快而筛掉慢点。

## 7. 本轮补充核对：部署与完整矩阵的具体约束

以下是本轮只读代码审计结果，不是新增实现或运行记录。

### 7.1 正式 system → 输入映射已经确定

以冻结的 `20260902-currentbase17-r1_c6620_systems_tiered_r1/matrix.json`、
`workloads.json` 中的三档 overrides 为准，而非根据图例名称猜测 security 参数。

| 正式行 | runtime security | 新 coalesced 输入 / 处理 |
| --- | --- | --- |
| Chunks | `full` | 无 WS 长流；保留 128-KiB native compressed chunks |
| Pages | `full` | 无 WS 长流；保留 4-KiB native compressed pages |
| Sabre (`ws-zstd3`) | `full` | `working_set_pages_content.zstd.streams`，原 `working_set_pages` 对应顺序 |
| SplitSnap− | `no-image-sharing` | `working_set_pages_content_private.zstd.streams`，原 private index；image 页仍在 private 内 |
| SplitSnap | `partial` | 同名 private payload，但使用 partial policy 自己的输入；不与 no-image 混用 |
| Full Dedup | **`partial`** | 经 canonical store 验证、预生成的 SplitSnap-format transfer view；不是另一份 monolithic full WS |
| AES-Go all-local | `partial` | 与新版 SplitSnap AES remote 相同输入和压缩表示，只改变本地驻留状态 |

正式 Full Dedup 的 `oracle_boundary` 是
`zero-cost pre-measurement SplitSnap-format transfer view`。不能因为名称叫
Full Dedup 就把 runner 改成 `security=full-dedup`：那会切换输入/恢复语义，且旧
restart helper 明确拒绝 `full-dedup + wsCoalescing`。新分支中其它显式
`security=full-dedup` 的候选支持，不等于这条正式行应使用它。
Figure9/10 仍从 canonical union 统计 Full Dedup 持久 footprint，不能用临时
transfer-view 的占用代替。Full Dedup 与 SplitSnap 的 transfer payload 可完全相同。

102 行 inventory 全部保留，但只对四个 coalesced 系统要求 WS layout/stream count。
Chunks/Pages 填 `ws_layout=not-applicable`、WS payload/manifest 为空、WS streams=0，
另记 native codec/chunk size；不能让“102 行全部 streams8”校验把 native 行误判失败。
AES all-local 是额外一行验收，不增加到 102 行六系统矩阵里。

### 7.2 不能照搬旧 restart / all-local helper

已核实的旧入口：

- `restart_snapshare_xl170_worker.sh` 会停止固定 tmux sessions，以及通用
  Firecracker/containerd/demux/resolver 进程；开启清理时删除固定 snapshots 目录。
  **即使传了新 relay binary/runtime_dir，也不会自动变成隔离启动器。**
- `run_figure11_aes_go_direct_all_r7_all_local.sh` 同样带旧服务停止与全局 cache
  备份/切换逻辑。新版只能填充、清理自己的 cache，不搬走现有 worker cache。
- 原 relay 的默认 `snapDir` 由 `os.UserHomeDir()+"/snapshots"` 得到。
  **本地候选现已有 `-snapshotsDir` / `-snapshotsScratchDir`**，分别设置 cache
  和退出时会删除的临时目录；不能继续称没有 CLI，也不能省略参数后使用旧默认。
- 旧 helper 将 endpoint 固定拼成 `IP:8080`；旧 inner window 的 invoker 和 outer
  runner 的 fetch-stats URL 也写死 `8080`。候选 `run-direct-window.sh` 已接受
  完整地址，但新 outer runner/readiness/counter/all-local 尚未完整贯通。

下次实现沿用并核对候选最小独立 launcher：显式配置 relay 完整地址、snapshot/cache
root、binary、工作/日志目录、net-pool 地址段、session/job 标识；这些接口已有
部分本地候选，**不是已验收的完整部署能力**。独立 snapshots 参数已传至
`ctriface.WithSnapshotsStorage`；不通过修改 HOME 来搬移其它 runtime 资产。
完整 relay 地址同时进入 invoker、ready probe、counter collector 和 all-local。
launcher 只管理自己创建的进程/VM/目录，不复用旧通用 kill 或清理函数。
复用现有 Firecracker/containerd 等服务之前，核实 socket/namespace/网络资源能否
共存；若不能隔离，需要报告具体冲突，不能擅自接管服务。

`-zstdFrameSize` 已从候选 CLI 移除，但旧 helper 在 **WS 或 native chunk 压缩
任一开启时** 都会传它。因此 Chunks/Pages 也会因未知参数启动失败；必须统一
清理所有六系统启动参数，而非只修四个 coalesced 行。`-j16` 是原恢复/UFFD
并发配置，`-zstdFetchers=8` 是 WS 下载/decoder 配置，二者不能互相替代。

### 7.3 不把旧 cadence 注释当成隔离保证

`run_figure9_16_direct_window.sh` 的 interval=1000ms 分支后台并发发请求；
interval>1000ms 分支改为前台，但仍按最初的绝对时刻表调度。若某次请求超时，
下一次可能紧接着开始；**5 秒 interval 不一定代表两次结束/开始之间有 5 秒空闲**，
更不能保证上一请求的异步清理已结束。
新 runner 应显式记录 cadence、scheduled/start/end、是否重叠；正式 10G 主矩阵
保留并说明旧 1-RPS 定义，隔离诊断/BW 的安静间隔另验，不静默改变采样口径。
本次不决定启动 BW；它与 C3/r12 trace 仍是需另行明确范围的发布依赖。

## 8. Corpus 落地顺序与中断边界

首版采用**独立目标 MinIO + 对象级选择性复制 + 离线转码**。交接已记录一组独立
端口/数据目录和准备任务；下一次 goal 先核实其占用、任务状态和磁盘，不能重复
创建。若旧任务不可恢复，再分配新 attempt；本文不把历史状态当作当前 readiness。
不用在线旧 MinIO 的内部数据目录复制或反向代理来代替新的独立 corpus。

1. 冻结原 17 个 revision、各自 tier/policy/input/index 与请求清单。只允许改
   新 endpoint/corpus 映射，不更换 base、函数输入、PFN 顺序或 private 分类。
2. 按清单复制必需的 snapshot metadata/recipe/WS index、shared base/image
   content/index，以及各 policy 对应的 native chunk namespace。保留 canonical
   raw-content store 供 oracle 验证。禁止全 bucket mirror 把旧 60-call aliases、
   两代 compressed WS 或无关 revision 一起搬入。
3. Sabre 按 full 模式、No-image/SplitSnap 按各自 private 模式转码。旧
   `.zstd.frames/.zstd.json` 仅为只读离线输入；本地解码恢复同一 raw，编码八流，
   一次 roundtrip 确认输出一致，原 index 原样保留。原 raw private/full 副本不作为
   新 compressed restore 的必需远端文件，也不因它存在而允许 runtime fallback。
4. 新 Full Dedup view 取已转好的 partial private payload + manifest + index，
   和原共享表示，经 canonical/PFN 对照通过才发布。不重复从另一旧 WS 重新编码
   出一个不同 baseline。旧 copy-plan 的固定“四个 private 文件”改为实际清单。
5. 全部 corpus/inventory 通过后，才生成新的 cold aliases；payload/index/manifest
   完整后最后复制 alias `snap_file`。基准 revision 自身已存在 snap_file 并不能
   证明新布局准备好，仍需独立的新 corpus completion marker。
6. 在别名清单与 102 行配置映射一致后生成新的 ALIASES_COMPLETE，附本次 layout/
   config 来源。不得复制旧 COMPLETE 或仅根据 alias snap_file 跳过格式校验。

再次核对：当前候选 `transcode/main.go` **已有逐 revision JSON journal、批次锁和
resume**；最终整批 report 仍最后发布。resume 核对来源/目标、旧 manifest、新
manifest、payload 尺寸和原 index；已复制的相同 index 不覆盖。它仍不是完整三档
controller，更不是对象内部的断点续传。中途失败留下无成功 journal 的对象时，
不能以“目标已存在”当作成功；要区分已提交 revision 与未提交 attempt。
对未完成的新目标使用独立 attempt 或只清理清单明确列出的本轮对象，不做全库删除，
不增加通用对象修复/复杂断点续传系统。最终完整报告/完成标记最后生成。
编码校验保留已有 SHA/CRC，不再额外多轮全库哈希扫描。

## 9. 下一次 goal 的执行顺序（本轮不执行）

1. 先读本文及 STATUS，保留现有未提交候选；保存精确代码来源，不把基线 commit
   当成候选 binary 的唯一版本。按缺口继续，不从零重写或重跑已完成的单元测试。
2. 复用已有独立启动/cache/endpoint 和准备 controller 候选，先核查已启动任务及
   成功记录；继续接通 oracle、aliases、实际 102 行 inventory、outer runner/collector。
3. 核实独立 corpus 后，审核已有小/大 WS 功能证据及其 binary，补齐顶层缺 WS
   失败、AES 同表示 all-local 0 GET 和生命周期检查，不无理由重跑已有 codec 冒烟。
4. 通过后跑完整 17×6 + AES all-local，再输出新 Figure9–13/汇总/复现说明。
   失败记录和退化保留；BW/C3/r12 未授权重跑时明确列为未迁移的发布依赖。

本轮交付到此为止：只读审计与文档更新，不启动上述步骤。设计选择已定，尚待
确认的是执行时的节点资源、隔离可行性和实测表现，而不是重新讨论八长 Range
还是 1-MiB 循环下载。

## 10. 再次本地核对：候选边界与开工前检查表

本节仅检查已有源码和配置，未重新运行测试、构建或连接远端。源码存在不等于
已部署；配置计划存在不等于对象已复制、端口已预留或实验已完成。

### 10.1 已有候选与仍需接通的部分

路径除特别标注外，相对 `experiments/currentbase17-streams8/`。

| 本地入口 | 已看到的候选能力 | 下一次实现仍需完成 |
| --- | --- | --- |
| `launcher.py`、relay/网络配置 | 独立 systemd unit、cache/scratch/log、完整 endpoint、netns/veth 名称前缀；不调用旧通用 kill | 与实际共享 containerd/demux/resolver 共存验证，配置生成、readiness、采集和正式调度贯通 |
| `run-direct-window.sh` | invoker 接收完整 endpoint；保留 60 calls 和原 cadence | outer runner、stats URL、调用成功/布局/样本 gate；显式冻结 invoker 来源 |
| `corpus_plan.py`、`config/20260910-r1/` | 三档 18 stores、102 配置映射，native 行 streams=0 | 核对 profile/system 精确集合与已有 uniqueness 证明；形成实际对象 inventory，而非只用计划行数 |
| `copycorpus/` | 默认只读清单；显式 copy 才写新目标；按来源 ETag/尺寸和 metadata 判断可续拷 | 复制总流程、容量检查、全局完成标记，以及与转码/view/aliases 衔接 |
| `transcode/` | 原 raw/PFN 顺序不变、逐 revision 成功 journal、resume 检查 | 中途未提交对象的 attempt 处理、三档 controller 集成 |
| `start-corpora.py` | 仅创建新命名 MinIO，检查计划/挂载和初步磁盘余量 | 全部新增 WS/aliases 的容量预算、恢复执行时按剩余工作核算空间 |
| `footprint/`、materializer | 新表示的统计/验证候选 | 真实新 corpus 输入、canonical/view 完成记录、最终新数据 |

`config/20260910-r1` 中 9560/9570/9580 是**候选端口基址**，不是已预留/可用的
承诺。运行前核实实际环境、计划是否已有未完成任务及其完成记录，不重复启动。
有外部 helper/binary 的依赖必须写入来源清单；不能仅凭当前分支的基线 commit
给所有未提交候选和 binary 标版本。

### 10.2 隔离不止一个 cache 目录

候选 relay 中 `WithSnapshotsStorage` 是 cache，`WithSnapshotsDir` 是 scratch；
`Orchestrator.Cleanup()` 会删除后者。两者必须分别指向本任务目录，互不包含，
不能保留 `/fccd/snapshots` 的默认值。`images` 查找依赖 cache 的同级目录；只读
复用原 immutable 资产，不以改变 HOME 来迁移整个运行环境。

`networkNamePrefix` 只隔离 netns/veth **名称**；还须显式检查 veth/clone IP 地址段
和路由不冲突，不能把不同名称当作不同网络地址。候选 launcher 返回 systemd
启动状态，并没有证明 relay 已 ready。下一次先确认完整 endpoint 的 readiness
及配置日志，再 reset 同一 endpoint 的 counters、发请求并收集；不能误读旧 8080。
退出只管理本任务 unit、VM、网络和 scratch，原服务不停止。

### 10.3 容量预算必须覆盖 aliases，而非只覆盖复制清单

`copycorpus.keep()` 排除了原始 WS 和两代压缩 WS。因此当前 scan 的 bytes
**不含随后生成的八流 WS**。`start-corpora.py` 的“scan bytes + 每对象 8 KiB +
16 GiB 余量”只是初筛，既不是完整预算，也不是 MinIO 实际磁盘开销的上界。

下一次创建前记录四项：选择性基础复制、新编码 payload/manifest、alias 副本、
转码临时文件/日志与对象存储实际余量。旧 alias 脚本对 revision 下除 `mem_file`
之外的文件执行对象级 `mc cp`，包括压缩 payload；**不是仅复制一个指针**。
按目前 85 个 distinct endpoint/revision × 60，是 5,100 个 alias revisions。
若保持此物化方案，alias 逻辑字节为各 distinct revision 待复制字节之和 × 60；
shared/native 全局 namespace 不因此乘 60。不要假设服务端 copy 自动零空间去重。

空间不足时先报告具体缺口与分批准备/保留策略，不删除旧 corpus，也不悄悄改为
多个请求共用一个 cache identity。改 alias 物化方式必须保持每次 cold restore
的原隔离/缓存语义，并记录测量期间是否有背景复制负载。

### 10.4 计时、限速和首版“不顺手优化”边界

- 候选 `GetWorkingSetContentSourcesManagedContext` 仍先解 private payload，
  再读 private index、base/rootfs 与 image sources；八流只作用于选定的压缩大
  对象，不代表 metadata/shared/lazy 全部八路并行。`getSharedWSSource` 仍是
  既有 raw shared 路径，不能声称全 restore 已具备相同的可取消 GET 保证。
- 首版保留上述 shared 路径及现有顺序、SHA、UFFD 安装方式，不顺带增加跨阶段
  并行、重采 WS、删校验或 direct-to-guest；如将来要改，独立归因。
- 八 decoder 是**每个 WS 对象/调用的并发上限**，不是节点全局仅八个线程。
  多次 restore 重叠时资源需求可叠加；保持主实验 cadence 并记录实际 overlap。
- 原正式 `run_figure9_16_direct_windows.sh::verify_platform` 检查 **worker
  `ifb0` 的入站 TBF**，并拒绝 backend 私网再有 TBF。不能沿用早期双节点
  文档的 backend 限速，也不能双重限速。10 Gbit/s 的实现位置、过滤器覆盖新
  MinIO 端口、burst/latency 应按正式配置冻结后复核，不在本轮改远端设置。
- 新库内 WS marker 是嵌套 wall time；八路 decode wall 含等待读取。Figure11
  的蓝段还有其它阶段，不相加重复计时，也不按总蓝段减 private wall 就称纯 GET。

下次 goal 的最短验收顺序仍是：**接通未完成工具链 → 独立实际 corpus/aliases →
小/大 WS 真 MinIO→UFFD、缺 WS 顶层失败、AES 同表示 all-local 0 GET → 完整
17×6 + AES all-local → 新统计/图**。先利用已有候选与测试证据，不重做已完成
的 codec 冒烟；每步完成以实际证据为准，不以计划文件名或服务启动返回码为准。

## 11. 本次冻结：已核对细节与剩余落地点（仅文档）

设计不再二选一：**一个 payload 内八条独立 Zstd 长流，每流一条长 Range 与一个
C1 decoder，直接写对应 raw mmap 区间**。按原始页数等分，不按压缩字节随意切割；
Zstd 标准 frame 仍存在，去掉的是应用层大量固定 1-MiB 小帧和逐帧任务调度。
本节基于当前本地文件，不重新验证远端状态或性能。

### 11.1 已有候选不应从零重写

| 入口 | 本地复核结论 | 下次实施要补什么 |
| --- | --- | --- |
| `snapshotting/zstdstreams/streams.go` | EncodeTo 页对齐写八流；DecodeWithStats 每流一次 open、C1 增量 Read，独立 raw 区间，末尾逐流 SHA | 审核已有小/大 WS 功能记录；补 release / 必需 WS 缺失的顶层验收 |
| `snapshotting/manager_streams.go` | remote 直接 Range→decoder；非 clean 仍 tee 整份压缩缓存，local 用同一 fd 的 SectionReader | 保留此计时/缓存边界；不得声称移除了所有 compressed buffers 或直接写 guest |
| `launcher.py` / `point_config.py` / relay | cache、scratch、netns 名称、8090 主端口、8091 cache HTTP 均已有候选；images 软链接解析已存在 | 新主配置与已验证 binary 对齐，贯通 readiness、counter、outer runner、all-local |
| `prepare.py` / `start-corpora.py` / `capacity/` | 已有三档复制/转码 controller 与基础+WS+最大 alias 批次预算；不再是只有扫描脚本 | 查既有任务/receipts；按实际新压缩大小及剩余工作核算 aliases 与物理空间 |
| `viewcopy/` / `finish-views.py` | 新 partial→Full Dedup view 复制及 canonical 验证编排；本地归档已有三档报告与完成标记 | 审核归档、核实现存对象；不要把 tier 报告直接当 Figure9/10 全局统计 |
| `inventory/` / `aliases/` | 实际对象/PFN/manifest 核对、102 行 inventory；85×60 aliases、剩余空间预算与按对象续拷已有候选 | 验收实际报告；`materialized=false` 的预检不能算 aliases 完成；接正式 runner |
| `cmd/relay/relay.go::handler`、`ctriface` | restore 使用 `r.Context()`；正常异步清理、shim refill 和失败归还 shim 已有 `WithoutCancel` 候选 | 正常路径修正不覆盖所有早退 StopVM 或 UFFD listener 失败；见 §11.6 |
| `validate_point.py::formal_plan/audit` | 已核对固定 inventory/materialized aliases、精确 slot/后缀/源对象、每流 manifest 长度；按 slot 区分 warmup | 接入正式 outer runner；其它 component markers 目前主要检查数量，还需 VM/调用关联 |
| `localcache/`、`local_aliases.py`、manager 统计 | 同表示 AES 原始 cache 工具、60 个本地 aliases 的不可变文件 hardlink 工具；strict-local 必须禁用 store 且零计数 | 本地只找到 gate3 配置，未找到其最终成功/释放验收包；先查既有任务，不重跑 |
| `negativews/` / `gate-invoke.py` / `validate_negative.py` | 本地已有 negative gate2 的独立验收报告：必需 WS 缺失、客户端 HTTP 500、未启动 UFFD/函数、shim 已释放 | 该 gate5 记录只覆盖进入 UFFD 前的失败；不代表所有晚期取消已安全 |
| `runtime_probe.py` / `/__snapshare/runtime-state` | 已观测 active requests、VM、cleanup、shim refill、WS cache write、UFFD handler；正式 validator 要求窗口末 idle | 接入窗口前/后及最终 counters 采集；不能把 idle 当作完整 startup readiness |

以上工具路径除 runtime 文件外均相对 `experiments/currentbase17-streams8/`。

### 11.2 已发现的具体接续风险

1. **不要让配置默认值选回旧候选 binary。** `point_config.py` 默认路径仍为
   `bin/relay-streams8`；落盘 AES gate2 配置使用 gate2，negative gate2 使用 gate5，
   local gate3 配置使用 gate6。三者不能被归为同一已验收版本。
   路径不是版本证明。下次统一冻结实际 source/patch、构建信息及各工具 binary；
   不把 `3f18475f` 基线 commit 单独当成未提交候选的精确来源。
2. **准备中断不等于重跑整个 controller。** `prepare.execute()` 认 stage receipt，
   工具另有最终 report/逐 revision journal。可能出现工具成功 report 已写、controller
   receipt 未写的中断；现有逻辑会因 report 已存在而停下，需核对后续接，不能删掉
   原目标重跑，也不能只见文件存在便补一个成功标记。receipt 存在也不是对象 inventory。
3. **本地已有全部 aliases 完成报告，但不证明当前磁盘余量。**
   `actual-layout.json` 有 18 stores/102 行，`aliases-complete.json` 有
   `materialized=true` 和 5,100 项；初始新增对象字节为 43,175,520,060，不能把
   这个逻辑字节数当物理占用。下次先查现存对象和剩余工作，不重复物化。
   `start-corpora.py` 启动前比较完整
   staged budget，部分复制后若无 receipt 再跑，会重复要求原始总余量；恢复执行应
   先核对剩余工作，不据此清理旧数据或换盘。准备 I/O 完成/退出后才采正式性能。
4. **日志自身一致不等于与计划一致。** 按 revision 配对已存在，不能重复列为未实现。
   当前正式 validator 已读取冻结 inventory/alias report，核对 `index=1..60`、
   `slot=0..59`、source→tag→slot、endpoint/system/profile 和每流 manifest 长度；
   不接受 `materialized=false`。这些不再列为待实现，但还未在完整正式窗口验收。
   非 WS 阶段及 teardown 仍应关联本次 VM，不只凑齐 component marker 总数。
   前 30 次按 slot 是 warmup，不能按完成先后切样本。
5. **strict-local 的零计数已有明确前提。** `ctriface/orch.go` 只在 remote 模式建立
   snapshot object store；manager 对 nil store 返回 `storage_disabled=true` 及零值。
   新 validator 要求此标记、local markers 和零计数；不支持统计的非 nil store 仍报
   不支持，不能伪装零值。local stream 的八个 `range_opens` 是 SectionReader，
   不是远端 GET。该保证只针对 snapshot/WS/lazy-page storage，不是 MongoDB、
   registry 等整次函数所有网络均为零。缓存准备阶段下载不在 restore 计时内。
6. **分阶段完成标记不能互相代替。** copies/transcodes → oracle views → 实际
   102 行 inventory → aliases → runtime gates → 103 个正式点 → 统计/新图。
   已有 `COPIES_TRANSCODES_COMPLETE` / `ORACLE_VIEWS_COMPLETE` 是局部契约，
   不能让 outer runner 把其中任意一个认作全系统准备或正式测量完成。

### 11.3 下次 goal 的最短交付路线

- 核对当前分支、已有任务和证据；保留旧服务，复用完成的准备工作，不重新录制 WS。
- 审核本地准备、实际 inventory / aliases 报告，核对存续状态后接正式调度，固定工具版本。
- 审核已有 AES / Image-Py 和缺 WS 负向记录；核实既有 local gate3 任务，补同表示
  all-local 0 GET 与生命周期验收。处理 §11.6 的失败边界，不重复已有小测试。
- 准备负载结束后跑 17×6 + AES all-local（每点 30 warmup + 30 measured），
  按旧口径整理 Figure9–13，新目录输出；保留失败和性能退化。
- BW/C3/r12 单列，用户下次明确纳入才扩展执行范围；不把旧图重新标成新实现数据。

**当前只完成文档核对，不执行上述路线。** 下次用户开 goal 后再继续实现。

### 11.4 统计报告不能直接混用：本轮确认的新细节

`cmd/materialize_full_dedup_oracle_ws/main.go` 的普通 batch 验证分支把全局 WS
按 hash 排序后编码，归档 `stages/oracle-tier512.report.json` 也明确写着
`global_working_set_order=ascending exact-content MD5 hash`。这是该 tier 的
canonical/view 验证附带统计，**不是 Figure10 的 17-workload 正式分子**。

正式入口 `experiments/currentbase17-streams8/footprint/main.go` 与旧
`snapshare-eval/analysis/figure9_10_currentbase17.go` 对齐：跨三档全局 union，
保留冻结 workload-manifest/PFN 的首次出现顺序，按新编码器重新计算压缩大小。
因此不将三个 tier 的压缩字节相加，不读取 hash-order 结果替代 first-occurrence
结果，不以 `ORACLE_VIEWS_COMPLETE` 代替新 Figure9/10 accounting report。
materializer 的 `-estimateGlobalWSOnly` 另有 first-occurrence 及 hash-order 对照，
也须显式记录输入集合，不能因为字段同名就混用。无需修改运行时页序来统一报告。

### 11.5 后续实验编排的最小验收契约

1. **接续而不是盲目重跑：** 先查既有 job/rc/report；本地归档已见 18 copy、
   9 transcode jobs 和三个 oracle tier 完成标记。远端当前状态本轮未核实。
   102 行 inventory、5,100 aliases 和 negative gate2 验收报告也已本地保存；
   local-cache/negative helper 已有候选，不能按早期清单从零重写。
2. **冷别名与 local 的区别：** `aliases/` 生成 85×60 远端物化副本，Pages/Sabre
   的共用关系保留；两个系统测量必须使用独立或已确认冷态的 cache。`localcache/`
   复制原始 AES 的 metadata/shared/recipe 所需 native chunks 与新 WS 表示，
   `local_aliases.py` 再按同一 alias report 准备 60 个 local revisions，不能重复
   恢复同一 cache identity 冒充原先的 60-alias 窗口。hardlink 只复用不可变文件、
   保持独立 revision 名称，不由此声称 footprint 获得额外去重。已有候选代码及
   gate3 配置，不等于本地已收到其准备 receipt 和最终运行验收。
3. **失败夹具：** 只在新 negative prefix 漏掉必需 payload、保留 manifest/index；
   不删原对象。除调用器非零退出/Load Error，还要确认错误来自 WS，未继续 lazy
   fallback 或函数执行，其它流退出后才释放输出。已有 fixture/部分 cache 无完成
   receipt 时先核对；两个 helper 不提供任意目录的通用修复/覆盖。
4. **正式窗口：** outer runner 需串联隔离启动、两个 endpoint readiness、版本/
   2.1GHz/SMT-off/10G-IFB 配置、counter reset、60 次调用、收集与验收。driver 等到
   HTTP 调用结束不等于 UFFD teardown/异步发布已结束；通过已有 runtime probe
   确认 active VM 列表为空、requests/cleanup/refills/cache writes/UFFD handlers 全零，
   并检查实际 StopVM/释放日志后，再取最终计数、切下一点。不能让前一点的尾部事件
   串入后一点或凭固定短 sleep 判断。`gate-invoke.py` 当前在 client 返回后立即取
   counters；正式 outer 不能直接沿用该时机，应 idle 后再采最终值。
5. **测量与准备分开：** aliases、全库 inventory、转码及 local-cache 下载结束后
   才采正式性能；冻结来源报告与代码 patch，不额外做多轮全库 SHA。成功率/退化
   如实记录，Figure13 仍排除 recipe，Figure11 嵌套 wall timers 不重复相加。

本节是后续实施清单，不是本轮已运行上述检查或实验的声明。

### 11.6 取消、释放和 readiness：仍须收口的精确边界

历史审计：其中早退 StopVM/未连接 accept 已有后续候选修正，见 §12；不要重复实现。
以下只读审计不修改源码；已修正常规路径与尚未覆盖的失败路径要分开描述。

1. **请求取消不能取消善后。** `runAsyncCleanup`、`scheduleRefill` 及失败归还
   shim 已使用 `context.WithoutCancel`；真实 Range 仍使用请求 context。旧 local
   gate2 的客户端成功且 store 禁用，但 StopVM/refill 出现 `context canceled`，
   已有 `INVALID.md`。不能把它算作 all-local/release 通过，也不能为了善后把
   所有下载重新改成 Background。后续只验收修正版，不覆写旧失败记录。
2. **早退清理仍有同类风险。** relay 的 function readiness、aux relay 分配/
   启动/readiness 失败分支仍调用 `StopSingleVM(ctx, vmId)`。若失败由客户端
   取消引起，传进去的 ctx 已取消。下次将这些必要善后统一放到不继承请求取消的
   清理路径；保留失败返回，不增加同步正常路径开销，不扩展成通用服务管理重构。
3. **UFFD listener 启动后、CreateVM 失败是另一个边界。** `LoadSnapshot` 先
   启动 handler，收到 `uffdReady` 后调用 CreateVM；handler 的 `AcceptUnix()`
   没有 context/显式外部关闭入口。如果 VM 在连接 UFFD 前创建失败，现有返回
   路径没有明确解除该 accept，`defer release()` 可能无法执行。这是源码暴露的
   风险，**不是已实测该挂起**。下次用最小生命周期接口在失败时关闭本次 listener，
   等 handler/写入退出再释放输出；不改 UFFD page-install 算法，不粗暴 munmap。
   缺 WS 的负向 gate 在 listener 之前失败，不能替代此路径的针对性验证。
4. **idle 是窗口收尾条件，不是 startup ready。** manager 在 goroutine 中加载
   image/rootfs hashes；HTTP 可响应、runtime 全零，不保证这些输入已加载。
   主 endpoint 和 cache endpoint 均须指向新 unit；后者有效路径为
   `/cached-working-sets`，根路径 404 不能判服务故障。partial/no-image 启动还需
   确认本轮预期的 image/rootfs 加载完成及配置一致，native 行按其实际需求验收。
   probe 观察超时只说明未收尾，不授权重启未知状态的任务。
5. **成功/失败证据不能只看一个状态字段。** negative gate2 的旧 wrapper 因
   gRPC 只报告 HTTP 500、不带原文字 body，写了 `expected_failure=false`；独立
   `negative-validation.json` 结合 relay 原因、零 invocation/UFFD 和 shim 释放
   才判预期失败通过。保留两份原始记录，不把所有 HTTP 500 都判为缺 WS。
   formal validator 目前仍保留部分旧 teardown error 例外；下一次必须以本任务
   VM ID/StopVM/handler 释放证据约束它，不能因 error 字符串匹配便一律忽略。

上述缺口并不改变八长流设计。下一次实现先补必要失败清理与 outer runner，再用
少量定向检查确认；不要重新开展完整五函数 codec 冒烟或多轮全库 SHA。

## 12. 最新冻结与接续清单（2026-09-10，仅文档）

本节覆盖上述历史状态，不改变 §1–2、§7.1 的设计。下次 goal 从此处接续，
不要根据旧条目的“仍未实现/正在运行”重写源码或重复发请求。

### 12.1 设计已确定，无需再选下载策略

- 编码时按原始页数分成八个连续区间，各自产生一条独立 Zstd 流，串存在一个
  payload 中；manifest 记录两套偏移。不能将已有单流压缩文件任意八等分。
- 恢复时每条流只打开一次长 Range reader，配一个 C1 decoder，增量写入同一
  host mmap 的不相交区间；八路完成后进入原 UFFD 安装。不是八个下载器循环
  请求 1-MiB 小片，也不是收齐整个对象后再解压。
- 固定每对象最多八路，不随 WS 变大增加流数；不是限制整个节点只有八线程。
  `-j16` 与八 decoder 是不同参数。所有 coalesced policies 都迁移，native
  Chunks/Pages、shared raw source、PFN/index、recipe 和安全分类保持原语义。
- `clean remote` 不为 WS 分配完整 compressed-cache 副本；非 clean miss 仍
  tee 压缩内容供异步缓存，local 仍从压缩文件解码。raw mmap、decoder window、
  UFFD copy 和 shared buffers 仍存在，不称 zero-copy 或免中间内存。
- 预期主要减少大 WS 的逐帧 GET/decoder 建立成本；小 WS 原来只有 6–7 帧，
  固定八流反而可能增加开销。性能由完整矩阵决定，不预设所有 workload 都更快。

### 12.2 已有代码和落盘证据：无需从零实现

以下路径相对 `experiments/currentbase17-streams8/`，仅说明本地已保存的事实。

| 项目 | 本地复核结果 | 不应推出的结论 |
| --- | --- | --- |
| 失败生命周期 | 已有早退取消修正；gate8 又增加 `confirmVMTermination`，核对精确 VM 进程缺席及该 VM 的 UFFD 完成后再释放；validator 要求终态证据 marker | 不是仅匹配 Internal 字符串即吞错；晚期 CreateVM/RemoveShim 失败仍须核对，见 §12.7 |
| 正式编排 | `run_matrix.py`、`point_worker.py`、`platform_probe.py` 已存在；60 slots、VM 关联 `RESTORE_COMPONENTS`、idle 后最终计数、103 点编排都有候选 | 不再写“outer runner 未实现”；源码存在不等于 103 点通过 |
| 定向测试 | gate7 历史日志保留；`verification/local-20260910-gate8/` 已保存 Go 文件级定向 race PASS 和 Python 23 tests OK | 本轮未重跑；Go 日志为 `command-line-arguments`，不能称整个 ctriface package 或全仓通过 |
| AES all-local | `aes-local-gate3/validation.json` 有一次同表示成功；payload 1,076,057 bytes；最终 store disabled、0 requests/bytes，VM/UFFD/异步任务全零 | 这是 gate6 功能证据，不是 gate7 的 60-call all-local 性能结果；旧 gate2 INVALID 保留 |
| 静态 footprint | `provenance/20260910-r1/footprint.json` 已保存，生成时间 `2026-09-10T09:27:11Z`，17 项、跨 tier 全局 union、新八流布局 | 不再写“仅构建/等 footprint 跑完”；它不证明当前远端 readiness 或 Figure11–13 完成 |
| 编排细节补丁 | `gate7-toolset-r2.tar.gz` 已收新版 runner/probes；`collect_final()` 先保存日志再等 idle，`online_policies()` 只读在线 CPU 对应策略；各有定向测试源码 | 不再列为尚未实现；本轮未运行测试或复核远端部署版本 |
| Figure13 collector / 静态图 | `provenance/20260910-r1/payload.json` 已有 `run_id=20260910-r1`、`layout=streams8-v1`、102 行；`results/20260910-r1-static-r1/` 已有 Figure9/10/13 的 CSV、PNG/PDF | 这些是既有产物，本轮只读；不再列为尚待运行 collector，也不代表 Figure11/12 或完整系统性能验收 |
| AES 正式窗口尝试 | `results/20260910-r1-gate7/` 现已有 60 行调用、relay log、platform-before 和失败收集记录；driver rc=0、60/60 exit0/reply_ok/native，但 controller rc=1，末尾 idle 失败 | 不是有效正式点或全矩阵完成；不能只凭调用成功纳入论文。具体 StopVM 语义风险见 §12.6 |
| 新版正式尝试 / 统计候选 | gate8 AES remote 已有 `started.json`、`ready.json` 和 controller argv；`metrics.py` / `aggregate.py` 已存在 | 本轮未验收 gate8 终态、不监测活动任务；不能重放该窗口，也不能称完整 103 点已通过 |

此前 inventory（102 行）、5,100 aliases、小/大 remote 和缺 WS 负向报告仍可接续。
本轮未查任何远端 job/PID；下一次获准后先查现有任务及其结果，不能重放已发出的窗口。

### 12.3 这次发现的具体剩余细节

1. **工具差异已有归档，补充准确版本清单即可。** `gate7-sources.tar.gz` 是较早
   源码包；`gate7-toolset-r2.tar.gz` 已包含 runner 的 `sudo -n` 调用、失败日志
   收集、在线 CPU probe 和本地 controller wrapper。只读核对 tar 内容确认上述
   文件已收录，不需再把同一修正当作待实现。后加的 `payload/` 不在该工具包里，
   下次单独记录其源码/构建来源。Python 编排变化不需要重编 Go 或全库 SHA。
2. **失败收集候选已工作，结构化诊断也已有 gate8 候选。** `collect_final()` 先
   保存 `relay-before-final.log` / `launch.json`，probe 失败保存 collection-error，
   finally 再取 relay.log。已有 AES 失败包证明这条收集路径确实执行过，不能再写
   “idle 失败就没有 relay 日志”。gate8 的 point_worker 新增只读 `diagnostic`
   动作，失败时保存 `unsettled-diagnostic-*.json`，明确 `settled=false`；不再
   从零补这段代码。它不替代正式 final，仍须核对实际失败包是否完整。
   收集失败绝不授权重新 reset、重放窗口或
   停止未知 VM；已有 `started.json`、job rc 和日志先审核后接续。
3. **取消 listener 不等于任意阶段都能终止。** 现接口只取消未连接的 accept；
   接受连接后按 VM 生命周期关闭。若 CreateVM 已连接后失败，且 shim 删除也失败，
   `LoadSnapshot` 中等待 `uffdDone` 仍可能等不到结束。这是源码风险，未实测；
   用最少定向证据确认正常/失败释放，不扩大成全局 kill 或强制 munmap。
4. **平台核对只读在线 CPU，实际状态仍待下次确认。** `online_policies()` 已从
   `/sys/devices/system/cpu/online` 解析在线 CPU 后解析各自 cpufreq；不再遍历
   离线 SMT sibling 留下的 policy 目录而触发 EBUSY。已有 platform-before 是
   此前 AES 窗口的 2.1GHz/SMT-off/10G-IFB 等检查记录，不代表本轮重查机器。
   probe 记录 qdisc 全文、未显式比较 burst；下次与正式配置一起核对。
   `point_config.py` 单独调用默认 binary 仍是无 gate 后缀的旧候选，正式 runner
   才显式选带 gate 后缀的 binary（当前候选默认 gate8）；运行版本必须显式冻结，
   不能靠默认值复用。
5. **缓存温度不是只看 remote 标签。** 新 runner 冻结为每点独立 worker cache、
   30 warmup、无系统级 drop_caches。旧 runner 支持每系统后全局清页缓存，
   旧结果还有 backend-cold-once 变体。需要按具体比较对象的配置/记录说明差异，
   不擅自清共享节点 cache，也不把不同温度/生命周期修正的结果归因成纯布局收益。

### 12.4 统计接口已明确，候选与待接通部分分开

- Figure9/10：使用新 `footprint.json` 的跨三档首次出现 union；不相加三个
  hash-order tier 报告，不把临时 Full Dedup transfer-view 当 canonical footprint。
- Figure11：按测量 slots 30–59（旧日志 index 31–60）各项取 median。
  蓝段为 download + GetUffdMemoryContent + GetWorkingSetPages + GetWorkingSetContent；
  Chunks/Pages 再按旧规则重分类 `max(0, insert − Sabre insert)`，从 restoration
  扣除同量。`ZSTD_WS_DECODE` 是嵌套诊断，不能重复加到蓝段。
  现有 `metrics.py` 已按 VM/alias 关联 `RESTORE_COMPONENTS`、download、relay E2E、
  preinsert/fault-handler，不再用匿名行配对；仍须验收 native/local 及正式数据。
- Figure12：每 workload 的测量调用取 relay E2E median，再归一化到 Pages。
  Mean 柱是 17 个原始 workload 标量的算术平均之比，不是 17 个比值的平均。
- Figure13：取新 inventory 的 compressed content payload，全部排除 recipe；
  native 行仍取原生对象集合。Mean 同样先平均原始 bytes，再除 Chunks 平均 bytes。
  依据旧 `plot_figure13_paper_payload.py`，不要另取含 recipe 的 logical_restore_bytes。
  新 `payload/` collector 已有候选：按 endpoint/key 复用尺寸查询，native 行按内容
  hash 去重，不仅按 PFN 去重；coalesced 行核对 frozen manifest 和 payload 尺寸。
  它读取 recipe/index 只是为了选择对象，不能把这些读取计入图中。完整 102 行
  `payload.json` 与 static-only 新图现已本地保存；下次先复核并复用，不再重复执行。
- 最终 fetch counters 覆盖整窗 60 次（包括 warmup），不是后 30 次；仅有这个总数
  不能除以 30 作为 measured mean，也不能替代 Figure13 的静态内容口径。

### 12.5 下次 goal 的待办顺序

1. 获准后先核实既有 gate8 AES remote 的同一 job/unit/最终记录，不重放原窗口；
   核对版本/磁盘/隔离，复用已完成 corpus/aliases、payload/静态图和 gate8 归档。
2. 复核已有 StopVM 终态/诊断候选，按 §12.7 收口剩余边界；验收同版本 AES
   remote/all-local。旧 gate7 仍 INVALID；需要修正版时另立 attempt，不覆盖旧记录。
3. 完整 17×6 + matched AES all-local，各 30 warmup + 30 measured，保留失败/退化。
4. 复用静态 collector，收口 metrics/aggregate 与 Figure11/12，形成完整 Figure9–13
   汇总及复现文档；只在新目录发布通过完整验收的包，旧结果不覆盖。
5. BW、C3/r12 仍单列，只有用户之后明确纳入才扩展执行。

**此清单本轮不执行。** 本次仅确定设计和接续细节，等用户另开 goal。

### 12.6 新确认的生命周期细节：StopVM RPC error 不等于 VM 尚存

本节保留 gate7 时的源码诊断；**gate8 修正候选已存在，最新接续见 §12.7**。
当时只读已有本地源码/落盘日志，没有连接远端或修正实现。它解释为什么不能把
“StopVM 有任意错误就保留所有记录”直接视为完整修复。

**已有证据。** 相对 `experiments/currentbase17-streams8/results/20260910-r1-gate7/`：

- `points/splitsnap-zstd3--aes-go-45000-45450--remote/invocations.tsv`：60 行、全部
  `exit_status=0/reply_ok=1/direct_native=1`；`window-rc=0`。
- 同目录 `relay.log` 有 60 条 `failed to stop firecracker-containerd VM`，错误为
  `Internal: forcefully terminated VM shim-19694-<n>`；第一条位于第 572 行。
- `controller-aes-r2/rc=1`；stderr 的末次观察是 requests/UFFD/cleanup/refill/
  cache-writes 全零，但 `active_vm_ids` 仍有 60 项，最终 idle 超时。这里是
  **relay vmMap 的记录数，不是已经证实 60 个 Firecracker 进程仍在运行**。
- 日志与 collection-error 已保留；无 POINT_COMPLETE，不能用于正式性能汇总。

**本地源码解释。** go.mod 固定的 firecracker-containerd replace 是
`github.com/vhive-serverless/firecracker-containerd@v0.0.0-20260304152437-f7f97e2dec10`。
该版本 `runtime/service.go:1876–1936` 中 `forceTerminate()` 先调用
`jailer.Stop(true)` 和 `cleanup()`，然后**无论这两步成败都返回**上述 Internal 错误；
真实 stop/cleanup 失败只另写日志。`terminate()` 在 graceful shutdown 未成功时
进入此分支。因此错误文字本身既不能证明“仍活着”，也不能证明“完全清干净”。
远端实际 containerd binary 是否等于此固定版本，本轮未核实；下次须对齐再归因。

当时 gate7 候选 `ctriface/iface.go::StopSingleVM` 一见 RPC error 就返回，跳过
`vmPool.Free()` 和 `workloadIo.Delete()`。其中 Free 先释放本 VM network 再删
vmMap，所以不只是最终计数器不好看：候选确实没有走完后续资源释放逻辑。
这与本地失败包相符，但不能由此断言底层 VM/网络现在的实际存续状态。

**下一次实现的最小处理契约：**

1. 分开“RPC 正常返回”“已确认强制终止”“仍运行或终态未知”。对该固定依赖的
   已知错误核对本任务对应 VM/进程或 shim 的退出证据、UFFD release 和清理结果，
   不匹配一个字符串就吞掉所有错误，不只凭控制端不可达/全局 UFFD=0 认定退出。
2. 已确认该 VM 终止且无写者后，完成该 VM 的剩余 network/map/IO/缓存善后，
   记录终止方式；仍运行/未知或真实清理失败则保留错误及 backing 状态，不强行
   munmap、广泛 kill 或把异常改成成功。
3. runtime 与 `validate_point.py::vm_lifecycles` 同步接受**有证据的终态释放**，
   而非只要求 nil RPC 或一律忽略 Internal。最少覆盖：强制终止已确认、真实
   StopVM 失败、UFFD 尚未释放三个情形；不扩展成通用服务管理重构。
4. 已有 r2 仅作诊断。修正版的正式 AES remote/local 在新 attempt 做同版本验收，
   通过后才续完整矩阵；不把旧 60 次调用成功作为放宽标准后自动转正的理由。

之后获准时定位既有任务：worker 配置中的 unit 为
`splitsnap-streams8-20260910-r1-splitsnap-zstd3-aes-go-45000-45450-formal-gate7-r1-remote.service`；
loader job/window 位于 `/users/Liquidz/streams8/20260910-r1/` 下的
`formal-jobs/` / `formal-windows/`，后缀均为
`splitsnap-zstd3-aes-go-45000-45450-formal-gate7-r1-remote`。
本轮未查其当前存续、未停止服务，controller 已记录退出不代表所有远端资源已回收。

### 12.7 本次补充复核：gate8 候选、静态产物与真正剩余项

本节仅来自本地源码和落盘文件；没有运行测试、构建、聚合、绘图或 SSH，也没有
查询/控制活动进程。此前实施产物保持原样，不能将其描述成本轮执行结果。

**已存在，不重复实现：**

- `ctriface/vm_termination.go` 的 gate8 候选仅接受 nil 或精确匹配本 VM 的已知
  forced-stop 错误，然后独立核对 `/proc` 中相应 Firecracker/shim 进程缺席，等待
  该 VM 的 UFFD 完成；之后才进行正常 StopSingleVM 的 network/map/IO 善后。
  `VM_TERMINATION_CONFIRMED` 与后续成功 stop 都是 validator 的要求；marker
  在 network free 前，不可单凭 marker 就宣称所有清理完成。
- `gate7-terminal-observation.json` 保存了 `2026-09-10T10:06:13.251729Z` 的历史
  观察：60 个 vmMap 条目、无相应 restored VM 进程、其余活动计数零。这补足了
  §12.6 当时缺少的进程证据，支持“残留记录”的诊断，不自动使旧窗口变有效。
  本轮没有重查远端依赖版本；不要由本机 go.mod 推断部署的 containerd 必然同源。
- `gate8-sources.tar.gz` 已含生命周期、正式 validator/失败诊断和 payload 工具；
  `gate8-toolset.tar.gz` / build-info 已存在。源码包是候选增量，不能仅用基线 commit
  复现整个 dirty worktree。其列表**尚不包含后加的 metrics.py / aggregate.py**，
  下一次补齐完整源码索引/归档；不需要重复全库 SHA 或重新转码。
- `metrics.py` 已按 VM/alias 关联 60 slots，取后 30 次 median；`aggregate.py`
  已实现 Figure9–13，正式路径要求全部 103 个 POINT_COMPLETE 并重新验证。
  `test_metrics.py` 中旧 gate7 失败包只用作解析夹具，不是有效性能输入。
- static-only 目录明确写着无新 latency 结论。Figure9/10/13 已有静态输出；
  这不能替代 102 个正式 remote 点及 matched AES all-local。

**之后 goal 优先收口的具体细节：**

1. **接续已启动的 gate8，而非再启动一遍。** 本地目录为
   `results/20260910-r1-gate8/`，controller 为 `controller-aes-r1/`；argv 仅选择
   `splitsnap-zstd3:aes-go-45000-45450:remote`。`started.json` 的 PID 是历史值，
   不是当前存活证明。远端 point/job 后缀是
   `splitsnap-zstd3-aes-go-45000-45450-formal-gate8-r1-remote`，位于既有
   `/users/Liquidz/streams8/20260910-r1/formal-jobs/`、`formal-windows/`；先查
   同一任务 rc/最终收集与 unit，再决定收集或后续点。本轮不监测、不接着跑。
2. **正常 StopSingleVM 的修正不覆盖全部 LoadSnapshot 失败路径。** 现
   `LoadSnapshot` 注册的 `vmPool.Free` defer 晚于 shim/UFFD release defer，
   因 LIFO 会先执行 Free；后者若 RemoveShim 失败，只记录错误并继续等待
   `uffdDone`。这仍可能在终态未确认前归还网络，或在已连接 UFFD 时一直等待。
   下一次核对这一顺序并作最小处理，不强行释放仍被使用的 mmap，也不吞错。
   这是源码边界，不是本轮实机故障结论。gate8 进程缺席检查只检查一次；若遇
   RPC 返回与 shim 退出竞态，应核对证据，而不是放宽成忽略所有 stop error。
3. **静态任务不得和计时窗口重叠。** `verify_prerequisites()` 已检查 prepare、
   views、inventory、aliases、footprint 的 rc，但未列入后加的 `payload-job`。
   已有 payload 报告可直接复用；若之后确需重跑，在窗口外完成并核对结束，
   不让新的 accounting I/O 干扰性能。这不是要求再跑一次 collector。
4. **统计候选需覆盖 native/local 以及完整发布边界。** 现 metrics fixture
   主要是 coalesced remote；补最少 native、strict-local、交错日志/缺字段和
   Mean 先平均原始标量再归一化的检查。不要把全 60 次 counter 除以 30。
   `aggregate.py` 目前先生成静态图才检查正式点，所以失败输出目录可能已有
   Figure9/10/13；目录/图片存在不代表完整结果包完成。完整发布应以全部点、
   聚合成功和最终说明为准，必要时先验证再发布，不原地覆盖旧图。
5. **只补当前版本真正缺的验证。** 文件级 Go race 与 Python 23 项旧日志已
   存在，本轮未重跑；不能说全 package/全仓已通过。先审核已有证据，再做最小
   必要验证和同版本正式窗口；不反复做五函数 codec 冒烟。完整 103 点、同版本
   all-local、新 Figure11/12 与最终复现包仍是后续实施验收，不在本轮执行。

## 13. 当前文档冻结：设计、已有候选与后续验收分开

本节依据本轮本地只读核对，覆盖 §12.7 的过期状态。未重新执行测试、validator、
聚合或实验，未查看 live tmux/PID/远端。已有记录不代表本轮新增测量，也不证明
现在节点或任务仍处于当时状态。分支仍是 `likun/splitsnap-sort`，原改动全部保留。

### 13.1 实现契约再次确认

1. 一个 coalesced WS payload，按原 raw 页序近似等分八段，在**编码时**生成
   八条独立长 Zstd 流并顺序写入；不是八个独立对象，也不是切割已有单流文件。
   `streams.go::EncodeTo` 使用 writer/Close，不再每 1 MiB Reset/EncodeAll。
2. manifest 保留每流 compressed/raw offset 与 length。远端每流一次长 Range
   reader、一个 C1 decoder，持续写自己的 host mmap 区间；各路无需等别的流
   下载完，但**全部成功后**才进入原 UFFD 安装路径。没有解压后 concat，也不是
   direct-to-guest；Zstd 标准 frame 仍存在，移除的是应用层大量小帧调度。
3. `streams.go::decode` 的 goroutine 生命周期覆盖整条流，CRC/长度/EOF 和每流
   SHA 保留，SHA 在各 goroutine 中完成后才返回。首版不同时删校验、换库、重录
   WS、排序页或新增跨阶段并行，避免把别的变化归因于八长流。
4. `manager_streams.go`：clean remote 不分配完整 compressed-cache slice；非
   clean miss 仍 TeeReader 收集压缩内容供异步发布；local 使用同一文件的八个
   SectionReader。三者均需要 raw 输出及 decoder 内存，不能宣称零中间 buffer。
5. 八路只针对 coalesced WS：Sabre full WS、No-image/SplitSnap 各自的 private
   WS、Full Dedup 的 `partial` oracle transfer view；shared raw sources、metadata、
   recipe、WS index、lazy 页和 Chunks/Pages native 粒度不借此改变（详见 §7.1）。
6. 每对象八路，不是每节点全局八线程；八路不能突破单链路带宽。较大 WS 可减少
   逐帧请求/初始化成本，小 WS 从 6–7 帧变八流可能增加开销，保留负向性能结果。

### 13.2 §12.7 中已有修正，不再列为从零实现

以下实验工具路径相对 `experiments/currentbase17-streams8/`。

| 项目 | 本轮看到的已有内容 | 后续界限 |
| --- | --- | --- |
| 失败清理顺序 | `ctriface/iface.go::LoadSnapshot` 已调用 `cleanupFailedVM`：取消 listener、移除 shim、确认精确进程缺席/UFFD 完成、最后 Free；teardown 用独立 30s context | 不再声称仍是两个反序 defer；最晚阶段失败的实际释放仍要少量定向确认 |
| 静态采集互斥 | `run_matrix.py::verify_prerequisites` 已把 `payload-job` 与其余五项准备/统计任务一起要求 rc0 | 之后若重跑准备任务，仍不能和计时窗口重叠；无需重复 collector |
| 收集与恢复执行 | `collect_window` 已在窗口结束后用一次 SSH tar 流收回小文件，失败不重放调用 | 收集耗时不算 WS GET；未知任务状态先查同一个 job，不能重启窗口 |
| 统计与完整发布 | `aggregate.py::main` 已先 `formal_data` 审计全 103 点，再建立输出目录；native/local/Mean/WS quality 有测试源码 | 本轮未运行聚合；仍须所有静态/动态图成功和最终来源说明，不以目录或单张图为完成 |
| 定向测试 | `verification/local-20260910-gate9/python-tests-final.log` 保存 32 tests / OK；已有文件级 Go race 日志 | 都是历史日志，本轮没有重跑；不称完整 ctriface package / 全仓通过 |
| AES 同版本记录 | gate9 AES remote/local 两目录均有 POINT_COMPLETE 与 validation：各 60 calls、60 stops、60 UFFD release；local counters 为 store disabled/零读取 | 不是需要再做的单次 gate；只是局部已有证据，不是完整 103 点或当前存续证明 |
| 源码归档 | `provenance/20260910-r1/gate9-source-complete-overlay-r2.tar.gz` 的列表已含 metrics、aggregate、runner、tests 与 runtime 候选 | 不再写 metrics 未归档；overlay 对应归档时源码，不包括本次文档改动、raw corpus 或所有部署依赖 |

gate9 AES 的 `validation.json` 记录 remote 的 60 次终止为 confirmed forced、local
为 graceful；调用成功不等于底层终止方式相同。这不否定已保存的终态验收，但旧版/
新版性能归因要说明生命周期和缓存温度差异，不能仅据这两点推断纯 codec 收益。
原 gate7 INVALID 保留，不按后来修正版自动转正。AES 记录入口见 STATUS。

### 13.3 本轮进一步确认的验收缺口

- **先决条件尚未绑定负向测试版本。** `verify_prerequisites` 仍读取旧
  `aes-negative-gate2/negative-validation.json` 的 accepted 标记；正常小/大与
  local 功能记录也来自较早 gates。它们可复用为既有功能证据，却不证明 gate9
  的失败清理。后续以当前 binary 补必要的缺 WS / 已连接 UFFD 后失败释放证据，
  不再启动五 workload codec 系列，不因 preflight 成功就略过这个版本差异。
- **POINT_COMPLETE 先于资源归档。** `run_matrix.py::point` 在写该标记后才停止
  自己的 idle unit、保存 `relay-resources.json`；完整聚合会检查资源文件。因此
  缺资源记录时接续收集同一 unit 的 journal，而不是再发 60 次请求。状态要区分
  “窗口验收完成”和“可发布证据包完整”，无需为此重写通用任务调度器。
- **资源计时不是解压计时。** 已有 `relay-resources.json` 是整个专属 relay
  unit 的 CPU time / rounded memory peak，包含启动、warmup、测量、收尾和
  收集等待，不包含其它 cgroup 的 guest/containerd。不能用它算八 decoder CPU
  利用率、单次纯解压开销或精确 RSS。若论文仅报告 latency，保留诊断说明即可；
  若要量化资源代价，再单独约定测量边界，不由生命周期总量推导。
- **统计来源须和正式 run 一起冻结。** `formal_data` 使用结果根的 frozen.json，
  但 `static_figures` 仍读取固定 `provenance/20260910-r1/footprint.json` 和
  `payload.json`。本轮没有发现被替换的证据；后续发布时核对这两份报告与 frozen
  inventory 的身份，列入结果来源清单。不得混入后来另一份 corpus 的同名报告。
- **Figure15 当前交付是 CSV。** `quality_summary` 与聚合已产出接口，定义
  extra UFFD events = handled − 1；whole-WS 比率不含 recipe，旧 private+recipe
  归一化另名保存。事件不是 unique remote pages。尚不能把此接口写成已生成
  新 Figure15 PNG/PDF；Figure13 继续完全排除 recipe，与旧诊断比率无关。

### 13.4 下一次 goal 的最短接续路线（本轮不执行）

1. 先核实已有 controller/job/unit、rc 和结果收集状态，不从旧的“2/103”或 PID
   推断现在进度。历史完整 runner 名为 `streams8_formal_gate9_matrix_r1`，结果根
   `results/20260910-r1-gate9/`，controller-matrix-r1；这是定位线索，不是存活证明。
   **不要并行启动第二个 controller、重建 corpus 或重放已经发出的窗口。**
2. 核对 source overlay/binary/toolset、实际独立环境和剩余空间；复用已保存的
   102 行 inventory、5,100 aliases、footprint/payload 和同版本 AES 记录。补上
   §13.3 必要版本/失败边界证据；不能干扰正在运行的测量，也不能停旧服务。
3. 接续并验收完整 17×6 + matched AES all-local，每点 30 warmup + 30 measured；
   保留所有失败、长尾和性能退化。只补真正缺少的点，不机械重跑已完成项。
4. 在新目录汇总 Figure9–13、WS quality CSV 与复现/来源说明，核对 raw/PFN/
   classification 未变和 Figure4 复用边界，完成逐项发布验收。
5. BW、C3/r12 仍为单列依赖；下次用户未纳入 goal 时不自动扩展，也不把旧结果
   重标成新实现数据。若要归因布局收益，只安排必要的同环境 matched 对照。

本轮交付是上述文档与核对结论。未创建/更新 goal，未执行接续，未控制既有服务。

## 14. 当前冻结：八长流方案不变，补齐生命周期与版本边界

> 历史审计，保留原定位链接。当前候选状态与后续待办以 §0 为准：其中整 unit
> shutdown、r3 源码/工具包和 frozen accounting 已有本地证据，不再是待实现项。

本节覆盖 §13 和此前 §14 中的候选状态及接续指向。依据本轮本地只读源码和
已落盘记录，**没有新测试、实机调用、远端检查或 goal 操作**。当前已有 gate11
候选，不再按旧“gate10 尚无负向包”的状态从零实施。下述 accepted 仅描述已有
证据文件，不代表本轮重验、完整系统完成或当前远端 readiness。

### 14.1 设计已经确定

- 同一原始 WS 按页序等分，编码为一个 payload 内的八条独立长 Zstd 流；每流
  一个长 MinIO Range reader + 一个 C1 decoder，直接写不相交的 host mmap。
  不循环下载 1-MiB ranges，不任意切割已压好的单流，不做解压后拼接。
- 保持 level 3、固定库版本、CRC/每流 SHA、PFN/index/分类/共享与 UFFD 算法；
  全部流成功后才交给 UFFD。clean remote、非 clean 压缩缓存和 local 路径的
  差别保持 §13.1 的说明，不声称零 buffer、零拷贝或提前安装 guest 页。
- 覆盖完整 17×6 + matched SplitSnap AES all-local。Sabre 用完整 WS；
  No-image/SplitSnap 用各自 private WS；Full Dedup 用已验证的 partial-format
  oracle transfer view。Chunks/Pages 保留 128-KiB/4-KiB native 粒度。
- 布局可能降低逐帧请求/初始化成本，也可能使小 WS 开销增加；不预设性能改善。
  BW、C3/r12 仍单列，不能以主矩阵迁移替代它们的验收。

### 14.2 本地证据更新：不是本轮新增实现

以下路径相对 `experiments/currentbase17-streams8/`。

| 已核对的材料 | 能确认的内容 | 不能据此声称 |
| --- | --- | --- |
| gate9 `controller-matrix-r1/rc`、`STARTUP_FAILURE.md`，三个点的完成/资源记录 | AES remote/local 与 Image-Go 有局部验收；Image-Py 当时在启动检查失败，未发窗口 | gate9 全矩阵完成，或现在仍有活动 controller |
| `verification/remote-20260910-r1/aes-negative-gate9/INVALID.md`、relay/diagnostic | 必需 WS 缺失按预期报错，但 RemoveShim 后相应预启动 VMM 仍存活，idle 超时 | wrapper 退出 0 就是负向清理通过；仅等待就能解决孤儿进程 |
| 同目录 `owned-vmm-retirement.json` | 历史精确清理记录列出该测试的六个 VMM 已退出 | 自动把失败 gate 转正，或证明其它旧点没有预启动残留 |
| `verification/remote-20260910-r1/aes-negative-gate10/{negative-validation,shutdown}.json` | 缺 WS 按预期失败；记录中该 unit 的六个 VMM 全部缺席 | gate11 的收尾也已验收，或整个矩阵完成 |
| `ctriface/orch.go::Cleanup` / `ShimPool.Cleanup`、`iface.go::StopActiveVMs` | gate11 候选拒绝新 refill、等待已有 refill、保留清理失败成员；先停池/活动 VMM，成功后才清网络/scratch，错误向上传递 | 所有并发停机时序已实机覆盖 |
| `verification/remote-20260910-r1/aes-negative-gate11/negative-validation.json` 与 `final.json` | 缺 WS 已有 accepted，释放 `shim-37617-1`，函数调用/UFFD 启动为 0，final runtime idle | unused 预启动池已退出；本地未见该目录的 `shutdown.json` |
| `verification/remote-20260910-r1/aes-cancel-after-load-gate11/{validation,final}.json` 与 relay log | 已有真实 LoadSnapshot 后请求取消；`shim-37617-2` 的 UFFD/VM 释放、idle accepted，`performance_sample=false` | CreateVM 内所有失败位置均已覆盖，或这是性能样本 |
| `verification/remote-20260910-r1/old-owned-vmm-retirement-gate11.json` | 55 个已结束测试遗留 VMM 的历史精确清理记录，`all_exited=true` | 本轮进行了清理，或现在不存在任何残留；也不能转正旧 INVALID |
| `verification/local-20260910-gate11/{vm-termination-tests,python-tests-final}.log` | 文件级 Go race、Python 37 tests / OK 的历史日志 | 全仓测试通过；`ctriface-focused-tests.log` 明确记录旧测试 API 签名不匹配导致编译失败 |
| `provenance/20260910-r1/gate11-build-info.txt`、`gate11-toolset.tar.gz`、negative 配置 | 已有构建信息与候选工具包/配置 | 完整、最新、可独立复现的 gate11 源码发布包；见下方归档差异 |

### 14.3 后续必须确认的几个细节

1. **先补收，而不是重测两个已经 accepted 的负向。** gate11 缺 WS 与晚期取消
   的本地包已存在，但本地缺整 unit 的 `shutdown.json`。两者共用配置指定的
   negative-gate11 unit，不能分别当成两个待启动任务。下次先查同一 unit 的
   `logs/shutdown-before.json`、`shutdown.json` 和最终 relay log；只补缺失收集。
   本轮未检查其是否仍运行或已经退出，不把本地缺文件解释成停机失败/尚未开始。
2. **idle 与整个池退出是两道验收。** 当前 launcher 已从本 unit 日志提取 VM IDs，
   写 ownership receipt，精确 stop 后核对这些 VMM/shim 进程缺席；runner 收回
   `shutdown.json`、`relay-shutdown.log`，aggregate 也要求 gate11 退出证明。
   不再把这些列为未实现。仍须核对实机最终证据；不能凭 unit 消失或 idle=0 放行。
   `refillWG.Wait()` 本身不响应 context；当前正式 stop 先 wait_idle，首版验收限于
   该路径，不声称支持任意负载中无界安全停机，不为此扩展通用 orchestration 重写。
3. **局部失败证据要准确命名。** `cancel_after_load.py` 发未完成 RPC body，在
   日志确认 VM 已 restored 后关闭连接；这是已连接 UFFD 后的请求取消，并非在
   CreateVM 内部注入失败。真实日志有 `context canceled`、对应 UFFD 结束、
   `VM_TERMINATION_CONFIRMED` 和 `Stopped VM successfully`。不能把它当成功
   invocation、性能点或所有失败位置的穷尽覆盖。
4. **源码、工具包、binary 必须同版本冻结。** 本地 runner 已默认 gate11，并要求
   同 binary 的缺 WS、晚期取消及整个池退出证明。只读展开已有 `gate11-toolset.tar.gz`
   可见其中 `run_matrix.py` 尚无晚期取消/退出前置条件，且不含 `cancel_after_load.py`；
   该归档与当前工作树不同。已有 gate11 build-info 也不能独自恢复 dirty 源码。
   下次先核对实际部署，再补完整源码 overlay/文件清单与最终工具包，保留旧包。
   不覆盖同名 binary 后复用 accepted，不增加多轮全库 SHA 扫描。
5. **冻结正式统计的真实来源。** `Matrix.__init__` 的 frozen.json 已保存 plan、
   inventory、aliases、版本和采样定义，但未保存 footprint/payload 内容；
   `aggregate.static_figures` 仍读取固定 `provenance/20260910-r1/` 的当前文件。
   仅 layout/run_id/profile 相同不足以证明与该 frozen run 完全同一输入。下次把
   实际使用的静态报告纳入结果快照或等价不可变身份对账，再绘图，不重做原 corpus。
6. **发布口径不因修生命周期而改变。** 新 binary 用独立 frozen 结果根，gate9
   三点保持历史局部结果，不改名补齐 gate11。旧测试残留的历史清理也意味着不能
   直接把两版差异全部归因于 codec。保留 102 行 inventory、5,100 aliases 和
   来源一致的静态报告；整包完成还需 POINT_COMPLETE 后的 shutdown/资源/图表。
   service-lifetime CPU/内存峰值不是 per-decode CPU/RSS；Figure15 当前为质量 CSV。

这里的缺口是收尾、身份及完整实验验收，不是重新讨论八长流与 1-MiB 下载队列。

### 14.4 等用户开 goal 后再执行的顺序

1. 查分支/dirty 来源、已有 controller/job/unit 和收集状态，先接收既有记录。
   gate11 定位配置为 `provenance/20260910-r1/aes-negative-gate11.json`，其中有
   精确 unit/binary/log 目录；远端工具候选为
   `/users/Liquidz/streams8/20260910-r1/toolset-gate11/`。不把配置存在当作未启动
   或 ready，不重建已有 corpus，不直接执行历史 gate9 接续示例。
2. 审核/补收已有生命周期证据；只针对真实缺口改代码/测试，不重跑五函数 codec。
   核对工具包与源码差异、静态报告身份、CPU/10G-IFB/磁盘/准备负载；保留旧服务，
   遇隔离冲突报告而不接管。原包中整包测试编译失败须如实披露，不写“全测通过”。
3. 冻结最终 binary/toolset 和新结果根，同版本完成 17×6 + AES all-local，
   每点 30 warmup + 30 measured；同一窗口收集中断只补收，不重复发请求。
4. 复用来源一致的静态报告，输出新 Figure9–13、WS quality CSV、失败/退化说明
   和完整源码/结果索引；核对统计定义与最终发布包。不混 gate9/gate10/gate11 样本，
   不将生命周期修正、缓存温度差异都归因为纯八流收益。

本轮到文档为止；不执行本清单，不更改已有 goal 或控制远端任务。

### 14.5 性能解释与最终验收速查

| 项目 | 需保留的准确边界 |
| --- | --- |
| 并行度 | 每个 coalesced 对象最多八个 reader/decoder；不是节点全局八线程，`-j16` 也不是 decoder 数 |
| 流式 | decoder 增量消费长 Range；不要求下载完整流后才解压，也不另加 1-MiB 任务队列；不承诺同一 C1 decoder 内网络和 CPU 完全并行 |
| “不用分帧聚合” | 不再有应用层大量小帧/解压后 concat；仍有八个标准 Zstd frame、边界 manifest、raw 输出 mmap 和原 UFFD copy |
| 可期待收益 | 大 WS 减少逐小帧 GET/初始化成本；原始等分不保证压缩字节/CPU 等分，小 WS 和慢一路仍可能限制收益 |
| fetch 计时 | private WS wall 嵌套在更大的 restore/download 阶段；包含等网络、解压和校验，不能把八路时间相加，也不能凭它推断完整 fetch 阶段组成 |
| 正式规模 | 17×6 remote + matched SplitSnap AES local，共 103 点、6,180 calls；每点 30 warmup + 30 measured，保留错误与长尾 |
| 网络/缓存 | 旧主矩阵 absolute 1 RPS，不预设请求隔离；BW 必须另验无重叠。10G 主矩阵点只有满足 BW 的隔离和计时边界才可复用 |
| 不重做/不混用 | 页序、分类、PFN、输入和 native Chunks/Pages 不改；原图不覆盖；静态容量、生命周期诊断、codec-only 与 E2E 数据分开 |

最终通过须同时有：同版本源代码/构建/工具包，完整 103 点原始与验收材料，
退出及资源记录，输入一致的 Figure9–13/WS quality，指标公式和失败/退化说明。
数据不支持性能改善也要保留；“所有 tests passed”或“图生成了”均不能替代这份清单。
