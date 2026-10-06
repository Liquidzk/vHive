# 八长流实现：逐项验收索引（设计与已有证据）

本索引按 `SPLITSNAP_SORT_IMPLEMENTATION.md` §0–14 的有效要求合并重复项，
不缩减目标。完整目标仍为 17×6 remote + matched SplitSnap AES all-local，
103 点、6,180 calls；新 Figure9–13、WS quality CSV 和可复现来源共同交付。
2026-09-11：完整103/103点及新图已核验。最终逐项证据以
[SPLITSNAP_SORT_FINAL_AUDIT.md](SPLITSNAP_SORT_FINAL_AUDIT.md)及正式图包的
DELIVERY_CHECKS/FINAL_DELIVERY为准；下文保留历史接收过程，不把历史部分通过
直接改标为最终通过。最新状态见 [STATUS](SPLITSNAP_SORT_STATUS.md)。

## 以下为历史设计与增量接收索引

当前用户goal continuation恢复完整实施；本轮gate12部署/源码身份已核实，同版
缺WS与真实LoadSnapshot后取消已通过，整个专属VM池7VMM正常退出材料已收齐。
同版检查通过，完整gate12矩阵已启动（无--only），当前进度以STATUS为准。
2026-09-11增量：gate12的SplitSnap全17点已按最终Matrix/metrics/quality及平台/
shutdown/resources身份规则接收，1,020calls、510measured，每点65所属VMM缺席。
`E/results/20260910-r1-gate12/RECEPTION_AUDIT.md`及r1–r17是本版证据；
完整17行`figure15-ws-quality-splitsnap17-reception.csv`已导出并绑定receipt来源。
随后gate12 Sabre AES-Go/Image-Go/Image-Py也已接收：各60calls，完整WS逐调用
插页4027/21239/32330与frozen一致，实际payload与manifest匹配，每点65所属VMM
缺席，来源r18–r20。随后VideoProc/VideoAn/AES-Py同样通过（r21–r23），完整WS
逐调用页数12884/18266/5535与frozen及实际payload匹配，每点65所属VMM缺席。
随后AES-NJS/Auth-Go/Auth-NJS也通过（r24–r26），完整WS逐调用页数
10250/4114/5973与frozen及实际payload匹配；每点65所属VMM缺席。
随后Auth-Py至Shipping八点也通过（r27–r34），Sabre全17点已收齐：
完整WS逐调用插页/实际payload匹配，60次VM/UFFD释放及65所属VMM缺席、6份平台
快照每点通过。与SplitSnap的17个profile集合及whole WS页数逐项一致。
此前SplitSnap− AES-Go/Image-Go/Image-Py通过（r35–r37），实际no-image-sharing配置、
逐调用全WS/private页数及payload与冻结输入匹配，各65所属VMM缺席，六份平台记录
通过；Image-Go正常收尾，未复现旧gate11该点的退出检查故障，不转正旧INVALID。
随后VideoProc/VideoAn/AES-Py（r38–r40），同样逐调用全WS/private页数与payload
匹配、65所属VMM缺席及六份平台记录通过。
此前AES-NJS、Auth-Go/NJS/Py、Fib-Go/NJS/Py（r41–r47），同样按最终验收规则通过。
本轮新增SplitSnap−最后四项（r48–r51），全17项的no-image-sharing配置、profile
集合及whole/private WS关系已统一核对。Full Dedup AES（r52）也通过：partial-format
oracle view、逐调用页数/payload与frozen匹配，65所属VMM缺席及6份平台记录通过。
本轮再新增Full Dedup Image-Go/Image-Py（r53内两点）、VideoProc（r54）、VideoAn（r55）、AES-Py（r56），
每点60calls及页数/payload/平台/退出记录通过；前六点实际endpoint与frozen corpus/job
映射核对一致，payload均与对应SplitSnap相同，符合partial-format oracle视图口径。
本轮新增Full Dedup AES-NJS/Auth-Go/Auth-NJS（r57–r59），同样逐调用页数/payload、
平台及完整退出记录通过，实际endpoint与frozen映射一致。随后本轮新增Auth-Py/
Fib-Go/Fib-NJS（r60–r62），同样通过逐调用页数/payload/平台/退出检查及endpoint
核对。本轮又接收Full Dedup最后五项（r63–r67），其全17项已齐。四个coalesced
系统各17项profile/security/endpoint/whole-private WS关系均已统一检查；68点共
4,080calls、2,040measured、32,640条流提前输出。随后Chunks AES首点（r68）也通过：
native128KiB/coalescing=false，chunk读取非零而coalesced标记为0，60次VM/UFFD释放、
65所属VMM缺席及平台记录通过。随后补收Chunks Image-Go/Image-Py/VideoProc/VideoAn/
AES-Py/AES-NJS/Auth-Go（r69–r75），八个native点各60次完整WS页数、native配置/端点
及非零chunk读取经`native-reception-r1.json`至`native-reception-r3.json`额外检查。
随后本轮补收Chunks Auth-NJS至Shipping最后九点（r76–r84），Chunks全17项齐全；
native补充检查延伸至r12，`native-chunks17-audit-r1.json`核对全profile集合、frozen
配置及与SplitSnap完整WS页数一致性。随后Pages前三点（r85–r87）通过，native补充
至r15，各60次完整WS页数、4-KiB/full/coalescing=false配置与frozen匹配，实际读取、
平台及退出核对通过。累计88点/5,280calls、2,640measured，同一controller进入Pages VideoProc。
Pages Image-Py E2E median7933.8944415ms完整保留，不改cadence/版本或筛样本。
两视频及Fib-NJS的
反向E2E差异原样保留，VideoAn/Auth-NJS/Fib-NJS的SplitSnap−快于SplitSnap也保留，
不筛选或换版本重跑。
Full Dedup AES与SplitSnap的payload相同而E2E中位数更高，原样保留；不把oracle
视图当作每点实测延迟的严格下界。Full Dedup全17项payload字节数与对应SplitSnap
相等，符合预生成视图口径；不能据此声称通用global gather已实现。
Chunks AES首点运行配置与frozen端点匹配；GetWorkingSetContent=0不代表native
下载/解压为0，具体运行计数/计时见接收记录。Pages余下14个remote点及AES local
仍待齐，不能用88点代替完整103点。
此前静态核对最终聚合输入：current plan==frozen plan，102 remote + matched AES
local范围保持；frozen_accounting核对102行对象尺寸/身份，68行coalesced均为八流。
这是输入与接口检查，不是全103点验收；没有新增外围代码或改变正式聚合guard。
完整103点和正式图包仍待齐，不是gate11重标。
旧失败点的17个网络已按创建日志/精确所有权退役，不转正失败点。实时进度见STATUS。
以下是原 gate11 材料的历史接收范围，不是 gate12 的通过证明：
Sabre第21–34点与第35点No-image AES经Matrix/metrics、身份/shutdown/
resources及插页/实际payload复核，通过；具体数值见
[增量接收记录](../experiments/currentbase17-streams8/results/20260910-r1-gate11/RECEPTION_AUDIT.md)。
下文较小完成数是历史证据范围；完整103点最终验收尚未达到。接口、计时、归因
边界见实施稿§0.4–0.6。最新后处理是`P/gate11-analysis-platform-r1.json/.tar.gz`：
包含下方全部既有增量，新增platform_evidence.py及正式聚合前的全点平台快照检查。
35点/210份节点记录通过，12项定向tests及archive compare通过；测量版本未改。
下方findings及更早增量为历史来源，不能再覆盖新版分析包。
本地Figure10排版增量单列`P/gate11-analysis-style-r1.json/.tar.gz`；r3运行版本不变。
此前analysis-methods含排版及METHODS导出；最新为
`P/gate11-analysis-findings-r1.json/.tar.gz`，另含findings.py/测试与正式FINDINGS.md
导出，直接叠加source-overlay-r3，不再覆盖旧style/methods包。8项定向tests通过、
tar compare rc0；测量版本/数据和图表计算公式未改。正式结果仍等全103点。

证据路径约定：`E=experiments/currentbase17-streams8/`，
`P=E/provenance/20260910-r1/`，`R=E/results/20260910-r1-gate11/`。
代码路径相对仓库根。表中“代码/局部证据”不等于全矩阵验收通过。

| 要求（实施稿） | 权威实现/证据入口 | 当前判定及最终还需核对 |
| --- | --- | --- |
| 一 payload，最多八独立长流；4-KiB 对齐，余页均分，空/小 WS 不填充（§1–2） | `snapshotting/zstdstreams/streams.go::EncodeTo/Manifest.Validate`；`TestRoundTrip`、`TestRejectInvalidInputAndManifest` | 代码/边界测试证据；正式各行仍与 frozen manifest 对账 |
| 固定 level3/window/checksum/C1，不使用小帧参数（§2） | 同包编码/解码选项；relay/converter CLI；`manager.go::ConfigureCompression` | 候选显式 streams8-v1/8；`TestStreamsRejectOldConfiguration`；六系统配置与真实 launch 均需完整验收 |
| 八独立长 Range 增量解码，不 ReadAll payload/不循环 1MiB/不 concat（§1–3） | `storage/minio_storage.go::OpenObjectRange`、`zstdstreams.decode`、`manager_streams.go` | 源码及 `TestOutputBeforeInputTail`；前五个已归档正式点各480流首输出早于末次读入，不外推尚未收齐行 |
| 任一路失败/父取消：关闭 reader，join 后释放 mmap（§3、§5） | `errgroup.Wait`、AfterFunc/Close、manager release；`TestFailureCancelsAndClosesEveryRange`、`TestParentCancellation` | 定向源码/测试证据；不声称共享 raw GET 已全局改写 |
| 压缩缓存收费/成功发布/淘汰；禁止失败半文件命中/淘汰后复活（§3） | `openCachedStreams/publishStreamCache`、epoch/deletion locks、registry；manager remote/local/corrupt/concurrent/expired tests | 局部功能与 race 覆盖；非 clean 仍有 compressed-cache buffer，未改成解压缓存 |
| 必需 private WS/index 错误失败，不回退 full/lazy；source/learning 合法缺失保留（§3） | manager policy/context 接口与 `RequiresCompressedWorkingSet`，LoadSnapshot 错误路径 | manager tests、gate11 缺 WS accepted；不能只凭底层 codec 错误验收 |
| VM/UFFD/预启动池退出与 backing 释放顺序（§11–14） | `ctriface/vm_termination.go`、iface/orch、`misc/vm_pool.go`；gate11 negative/late-cancel/shutdown | 同版负向包已收7所属VMM退出；正式已归档点有65所属VMM退出。仍需全部103点；idle 不能代替 unused pool 退出 |
| 冻结 raw/PFN/分类，离线转码，不重录/排序（§4、§7–8） | `E/transcode/main.go` source decode、原 index bytes.Equal、roundtrip；`P/preparation-reports.tar.gz` | 已有转码来源/receipt；新 inventory、运行插页数与 frozen input 逐行核对，不新增全库 SHA |
| 六系统映射和 Full Dedup oracle 语义不变（§7.1） | `E/point_config.py`、materializer/viewcopy、`P/actual-layout.json`、frozen plan | 68 coalesced +34 native；Full Dedup 用 partial-format view，canonical union 作 footprint；102行正式结果待齐 |
| 三档/独立namespace/aliases、准备负载不与性能窗口重叠（§7–8、§10） | plan、preparation/inventory/aliases receipts；`R/platform-preflight.json`；runner preflight | 已有18 stores/102行/5100 aliases和六项准备任务rc0；每点保留独立cache/launch/platform证据，不能接管旧服务 |
| 小/大 WS 真 MinIO→UFFD 与同表示 all-local 0 reads（§5、§9） | `E/verification/remote-20260910-r1/{aes-gate2,imagepy-large-gate1,aes-local-gate3}` | 旧局部功能证据保留版本；gate11正式 AES local 在103点矩阵内，尚须完成，不能由旧功能点代替 |
| 每点60calls、前30warmup/后30measured、成功+原始失败记录（§11.5、§14.5） | run-direct-window、runner/validate_point，invocations.tsv/calls/relay/final/platform | controller同一任务推进；不重放已结束窗口，不混gate9/10/11；部分已归档，不完整 |
| metadata/index/recipe与payload计数分开（§4–5） | MinIO fetch classes、inventory/payload工具、validator exact manifest | Figure13只计实际压缩payload；object/stream/range-open/HTTP attempts不混用。正式读取bytes仍待全矩阵对账 |
| Figure9/10代表性17、跨档union/首次出现顺序、原分母（§4、§11.4） | `E/footprint/main.go`、`P/footprint.json`、frozen accounting | 新静态报告已存在；正式图须读R/frozen，不能读可变报告或用hash-order tier总量替代 |
| Figure11/12 keyed组件、measured medians、Figure12/13 ratio-of-raw-means（§12.4） | `E/metrics.py`、`aggregate.py`与对应tests | 前五个完整点重新解析通过；不同VM日志不串线，嵌套WS wall不重复相加。所有系统/AES local未齐，正式Figure11/12尚未生成 |
| WS质量保留事件/长尾，校对whole/private页数（§12–14） | `quality_summary`、frozen inventory、Figure15 CSV接口 | 前五点60slot/页数对账通过；额外UFFD事件不是唯一远端缺页，legacy recipe归一化单列。全17行CSV待齐 |
| 来源/工具与运行版本一致，原图/结果不覆盖（§13–14） | `P/gate11-release.json`、source-overlay-r3/toolset-r3、R/frozen | 本轮tar compare（排除文档）rc0，运行代码未变；最终源码/结果/图索引与发布包仍待完整验收 |
| 外部invoker及运行依赖不能被relay源码包掩盖（§4、§10.1） | `P/external-direct-invoker.json`、其源码archive/build-info；既有运行配置/平台记录 | 新补loader binary与本地/原currentbase17 receipt一致；source为vSwarm a096cc4。不是逐调用校验或位级重建证明；完整外部依赖索引继续核对 |
| 外部worker二进制/配置及分析环境（§4、§10.1） | `P/external-worker-runtime.json`、worker config archive/build-info、`aggregation-environment.json` | 四个worker binary与本地保留副本同SHA，未更换；两个modified=true组件不能只靠嵌入commit重建，复现保留精确binary。分析依赖版本已记录，未安装/修改软件 |
| 源码恢复/结果复现入口（§13–14） | `E/REPRODUCE.md`、`P/reproduction-source-check-r1.json` | base→r3→analysis-findings-r1已在独立临时目录实际恢复，七个脚本/plan cmp相同、聚合CLI help成功；非位级binary重建/完整图重绘，后者仍等103点 |
| 性能/资源解释与BW/trace边界（§10.4、§14.5） | final README/CSV、service resource record、每slot时刻 | 主矩阵absolute1RPS不是隔离BW；service-lifetime CPU/peak不是per-decode CPU/RSS。BW/C3/r12不在本次103点完成声明内 |

## 已完成的离线接收检查

本轮使用现有 `Matrix.validate`、`parse_metrics`、`medians`、`quality_summary`
重新读已完整归档的前五点，另核对 shutdown/resource 与 config 身份，无新函数调用：

| SplitSnap remote | 后30次 E2E median (ms) | WS content median (ms) | 全WS/private页数 | 额外UFFD事件 median / max |
| --- | ---: | ---: | --- | --- |
| AES-Go | 42.2017435 | 10.5245 | 4027 /1516 | 4.5 /594 |
| Image-Go | 674.573161 | 11.806 | 21239 /2470 | 3 /12 |
| Image-Py | 2155.9406505 | 70.48 | 32330 /27702 | 10 /458 |
| VideoProc | 147.0638685 | 27.456 | 12884 /8234 | 10.5 /59 |
| VideoAn | 519.373273 | 36.679 | 18266 /12956 | 7 /18 |

同一接收检查随后扩到八个已完整归档点，新增 AES-Py、AES-NJS、Auth-Go，全部通过。
本轮又覆盖前13点，新增Auth-NJS/Auth-Py/Fib-Go/Fib-NJS/Fib-Py；同版调用/流/
VM/UFFD和shutdown/resources身份、最终metrics/quality均通过。每点65所属VMM
全部缺席，保留Fib-Go等长尾，不把额外UFFD事件当作唯一远端页数。
随后Currency/Email两点也完成同一接收验收，共15点/900calls；二者各480流有
末次读入前输出、60次VM/UFFD释放和整unit65个VMM缺席。measured E2E median
分别54.469758/46.940479ms，WS content分别14.2395/14.463ms；仍只是局部结果。
本轮完成全17个SplitSnap remote的同样复核：1,020calls、510measured，8,160流
均首输出早于末次应用读入，17个unit各65所属VMM缺席，全部WS/private页数匹配。
Product Catalog/Shipping的E2E median为46.8157195/41.040435ms，WS content为
12.827/13.0625ms，额外UFFD事件median/max分别3.5/11、2/14。未筛除长尾。
Sabre17份frozen行也均满足完整WS/index/raw页数一致和八流布局。Sabre AES正式
窗口现已完整归档并追加验收：security=full/9562，60calls各插入4,027页，480流
提前输出，320,274,120payload bytes精确对账，60次VM/UFFD释放与整unit65VMM
缺席。后30次E2E median51.38104ms、WS content21.3895ms；总计18点/1,080calls。
其它Sabre workload尚待对应runtime验证，不由这一个点外推全部通过。
之后Sabre Image-Go/Image-Py也完成同样完整验收：各60calls、480流提前输出、
整unit65VMM缺席，完整WS21,239/32,330页，每调用payload8,590,812/26,670,173
bytes精确匹配frozen与读取计数。E2E median734.131546/2177.880111ms，WS
content60.2105/86.237ms；累计20点/1,200calls，不外推尚未验收系统。
以上只是当前收集/统计接通证据，不是与其它baseline的收益比较，也不是最终图。
后续最终聚合仍重验全部103点，不能通过缩小plan绕过缺失点。

当前额外本地测试记录：`E/verification/local-20260910-gate11/snapshotting-final-focused.log`
为 snapshotting focused race PASS（1.434s）；已有 codec 六组测试和Python40历史日志
也已定位。`ctriface`整包旧测试API编译失败记录保留，不能写“全仓测试均通过”。

外部worker运行依赖只做只读核对：Firecracker v1.13.1，containerd/shim/demux
与现有本地副本同SHA；三份运行配置已收回。containerd/shim的嵌入构建记录均为
modified=true，不能假装源码commit已足以重建它们；它们是原平台保留的依赖，
没有在八流迁移中重建。完整guest rootfs/kernel/镜像仍沿用既有输入来源，未新增
大镜像/全库SHA遍历；该版本记录不证明历史每个调用前都重新验证过binary。

## 完成判据

静态图外观已目检Figure9/10/13；Figure10长纵轴标题修为两行并固定四个刻度，
另存`R`同级`20260910-r1-static-r2/`。两份CSV与static-r1逐字节相同，5个聚合/
accounting tests通过；增量源码已单独归档。r2只是静态预览，最终正式包必须仍从
frozen输入、完整103点生成并目检；不能把排版增量当测量版本变化而重跑窗口。
新增`E/METHODS.md`随aggregate输出复制/链接，说明Figure9/10实际大小与模型、
Figure11逐组件median/重分类/残差及其它统计和计时边界。static-r3已验证文档
复制及两份CSV与r2一致，5tests通过；最新analysis-methods增量归档compare通过。

只有表中所有本次范围项得到其完整范围的权威证据，且103点原始/退出/资源、
全部新图CSV/PDF/PNG、WS质量、来源/公式/退化说明与可复现包齐全后，才标记完成。
“矩阵已启动”“部分点测试通过”“静态图已有”均不足以结束goal。
