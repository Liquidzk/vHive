# 八长流迁移实施状态

## 当前：103点和Figure9–13已经收齐并核验（2026-09-11）

gate12完整103/103点、6,180calls/3,090measured，controller rc0；Python3374747已退出。
四个coalesced系统与Chunks/Pages各17个remote点，加matched AES all-local。
本轮从冻结base+source-r1的真实恢复目录运行aggregate，完整重验103点后生成新图；
8份CSV、Figure9–13各PNG/PDF、METHODS/FINDINGS均齐，数值及图形检查通过。
结果：[正式图包](../experiments/currentbase17-streams8/results/20260910-r1-gate12-figures-r1/README.md)。
审计：[FINAL_AUDIT](SPLITSNAP_SORT_FINAL_AUDIT.md)；复现：[REPRODUCE](../experiments/currentbase17-streams8/REPRODUCE.md)。
逐点65所属VMM缺席及618份平台快照通过，local store禁用/零远端读取，AES local
E2E median33.2669555ms。原relay/containerd/demux及58个原网络命名空间末次只读
检查仍保持，见gate12-final-original-services-r1.json；没有为收尾停止原服务。
保留VideoProc/VideoAn/Fib-NJS相对Sabre退化，以及全部WS长尾。此次是同轮系统
比较，不是旧帧/新流的纯布局A/B。BW/C3/r12/IAA未扩入，旧图/输入/失败点不覆盖。
本轮收尾仅补运行后文档增量与交付索引，没有再改运行代码、构建、发请求或清理服务。
最终交付清单为图包中的FINAL_DELIVERY.json；源码/文档归档分别保留，不混写运行版本。

## 以下为历史阶段记录

最新指令核对：2026-09-11。**用户goal continuation恢复完整实施。**
目标仍17×6 + matched AES all-local、新 Figure9–13/WS quality与复现来源。
上一轮文档边界已被覆盖，实施稿§0的设计/计时/归因约束不变。

## 历史：gate12累计88点已接收，五系统各17点齐全，Pages前三点通过

同一Python3374747本轮实查live（elapsed5:30:15）；SplitSnap、Sabre、SplitSnap−、Full Dedup
和Chunks各17点、Pages前三点已收齐，进入Pages VideoProc。累计5,280calls / 2,640measured用最终Matrix/metrics、
输入页数及平台/shutdown/resources身份规则离线接收；每点65所属VMM缺席。
此前收齐Full Dedup最后五点Fib-Py/Currency/Email/Prod. Cat./Shipping（r63–r67）；
旧r53包含两个Image点。四个系统各17项profile、安全配置、endpoint及whole/private
WS关系已统一核对，68点共32,640条流提前输出。Full Dedup全17项的payload字节数
均与对应SplitSnap相等，符合预生成oracle视图口径。
随后Chunks AES也完整接收（r68），native 128KiB/coalescing=false，实际chunk读取
非零、没有八流WS标记；60次VM/UFFD释放及65所属VMM缺席通过，forced=0。
本轮补齐Chunks Auth-NJS至Shipping最后九点（r76–r84）；十七个native点各60次完整WS
插页数、配置及非零chunk读取已额外核对，见`native-reception-r1.json`至`native-reception-r12.json`。
`native-chunks17-audit-r1.json`还核对完整profile集合与frozen一致，whole WS与对应
SplitSnap全17项一致；实际chunk读取合计1,617,763次、69,553,465,827bytes。
随后Pages AES-Go/Image-Go/Image-Py（r85–r87）通过，native补充检查至r15：4-KiB、
full/coalescing=false，各60次完整WS页数匹配，实际page对象读取非零，平台/退出均通过。
Image-Py E2E median7933.8944415ms、native预插入5374.5299555ms完整保留；
不当作隔离带宽/纯解压实验，不改变cadence或筛选结果。
没有将coalesced payload字段为0误解成无下载/解压，详见
[Gate12接收记录](../experiments/currentbase17-streams8/results/20260910-r1-gate12/RECEPTION_AUDIT.md)。
本轮不改测量代码/工具/输入，不重跑冒烟、不重编码，不增加外围实现。
上一goal turn新增接收和native检查/结果表，属于progress；本goal turn补齐Chunks
最后九点、逐调用页数/计数及全17项集合检查，同样属于progress。Chunks AES E2E
median120.5161135ms，native预插入97.6924925ms；
0 coalesced decode不能解释成零下载/解压。仍需Pages余下14个remote点、AES local、
完整新图及最终审计。此前已核对当前plan与frozen一致、完整103点范围、
102行静态payload对象尺寸/身份及68份coalesced输入的streams8-v1布局；没有运行
未收齐的正式聚合或新增实现。此前本版No-image Image-Go正常收尾通过，未复现旧gate11该点的
procfs退出检查故障，旧INVALID保留、不转正。期间同一controller及其收集/stop
子进程持续实查，无重启或重放。三项本轮SplitSnap E2E中位数比Sabre高：VideoProc
183.773 vs147.412ms、VideoAn567.188 vs530.227ms、Fib-NJS131.343 vs128.094ms，保留原值，不筛选、不作
单一原因归因，也不把较慢结果当作启动新版重跑的理由。
VideoAn的SplitSnap−中位数548.806ms低于SplitSnap的567.188ms，Auth-NJS为
69.138 vs72.419ms，Fib-NJS为128.492 vs131.343ms，三项完整保留，不预设
image共享的E2E收益一定为正。
Full Dedup AES中位数47.817ms高于SplitSnap的40.174ms，原样保留；预生成oracle
view不等于实测延迟必然下界，也不能从首点外推全17项。未新增layout或metadata优化。
完整103点和最终新图尚未完成，后续继续同一任务，不按旧完成数重启。
Fib-Go后30次额外UFFD事件median3/max607，6次超过100事件，完整保留；
全WS/private页数4906/1756仍逐调用匹配，不据此重录WS或更换测量版本。
完整17行WS质量已由验收receipts用现有writer导出：
`results/20260910-r1-gate12/figure15-ws-quality-splitsnap17-reception.csv`，
来源`ws-quality-splitsnap17-reception.json`；frozen profile集合/17行/510measured匹配。
全17个workload共10次>100额外事件，均保留。这是WS质量子表，不是完整103点图包；
最终聚合仍从原始日志重建，不以该子表替代其余系统或AES all-local。

### 已完成的版本准备（历史证据，本轮未重复）

- gate11 controller终态rc1，旧tmux句柄缺席；旧35点保留，不重标或混入gate12。
  原worker relay4180610、containerd4180024、demux4180059本轮实机核实存续。
- gate12 worker binary SHA与release一致，三节点各43个归档工具文件逐字节一致，
  本地源码overlay105个非Markdown成员一致；证据
  `provenance/20260910-r1/gate12-deployment-source-check-r1.json`。
  tar-d的owner/group/mode差异单列，不把它误报成源码不同或偷偷改权限。
- 旧失败点65VMM退出记录保留。新增`retire_gate11_networks.py`按创建日志精确
  验证sstruvmns1–17、veth地址/peer、无所属VM/namespace进程后，只退役对应网络/
  nft链；原namespace与三个服务的PID/cmdline保持。证据在旧失败点
  `diagnostic-network-retirement.json`，不转正INVALID，不删缓存/结果。
- gate12缺必需WS验证通过：客户端失败、精确missing-payload原因、0函数执行/
  UFFD安装、VM释放。真实LoadSnapshot后取消也通过：VM/ UFFD释放、runtime idle，
  不计性能样本。正常launcher.stop收齐7所属VMM缺席、unit inactive/Result success。
  本地 `require_versioned_negative_gate(...,'gate12')`通过，路径
  `verification/remote-20260910-r1/{aes-negative-gate12,aes-cancel-after-load-gate12}`。
- 完整矩阵在tmux `streams8_formal_gate12_matrix_r1`，pane3374741 /
  Python3374747，完整命令无`--only`，本轮实查live。结果根
  `experiments/currentbase17-streams8/results/20260910-r1-gate12/`，attempt
  `controller-matrix-r1`。先查同一进程/job/rc，不重放已开始窗口。

原raw/PFN/corpus/aliases按身份复用，没有重复编码/全库SHA或新的五函数codec冒烟。
运行二进制与已冻结工具不再改变。上一轮文档核对改变了接续依据；本轮完成精确
资源收尾、同版部署/源码证明、两项负向和整池退出以及完整矩阵启动，是progress。
完整103点、新图及最终逐项发布审计仍未完成；goal保持active，不以启动/局部验证收口。

## 上一轮本地文档核对：已有候选，不从头重写

- 已核对 codec、manager、Range 接口和 CLI：一 payload 最多八独立长流，每流
  一个长 Range/C1 decoder，直接写不相交 raw mmap，完成并校验后交给 UFFD。
  保留原页序/PFN、共享策略及 SHA；不循环1-MiB请求、不做最终解压拼接。
- `run_matrix.py` 已默认 gate12，aggregate 接受 gate12；本地已有
  `provenance/20260910-r1/gate12-release.json`、完整 source/toolset-r1 归档及
  同版负向配置。未执行测试/归档比对/远端部署核对，不将文件存在写成验收通过。
- gate11 No-image Image-Go 的 `diagnostic-retirement.json` 已落盘，记录
  MainPID=0、service failed、65所属VM及 remaining_processes=[]。它明确标为
  diagnostic_only，网络/cache未动；不转正旧失败点，也不能当正常 shutdown 证明。
- 因此下节“relay126349仍active”“gate12尚未接入”是过期快照。下次获准先核对
  实际任务、部署、同版负向/晚期取消/整池退出，以及网络资源的精确所有权。
  不重放已结束窗口，不按通配符清理，不把 gate11 局部结果重标 gate12。

没有本轮远端存活/端口/带宽或完整矩阵完成数结论。完整103点及新图尚不能由这些
局部材料宣告完成；本次仅完成文档与设计确认。

## 历史：gate11矩阵rc1退出；当时gate12 procfs修正仅本地build

最新实查覆盖下方live快照：Python3318838/pane3318832已不存在，
controller-matrix-r1/rc=1。前35点/2,100calls已验收保留；第36点No-image Image-Go
60次调用退出0，但收尾active_vm_ids残留`shim-126349-53`，未通过正式验收。
专属relay PID126349仍active；只读核实该VM的Firecracker/shim已不存在，UFFD已退出。
错误是/proc/128022/cmdline并发退出返回ESRCH，而Go扫描仅忽略ENOENT，导致
vmPool.Free未执行；Python launcher也有同类缺口。详见该点INVALID.md。

本地已修Go/Python扫描中的ESRCH处理，权限/I/O错误及所有实际VM/UFFD验收不放宽。
Go文件级race PASS1.014s，Python launcher7tests OK；bin/relay-streams8-gate12
build退出0，尚未部署。记录P/gate12-procfs-fix-local.json；未停止或改动远端服务，
没有重放窗口。下一步精确收尾失败unit、接入gate12版本/来源/工具包与同版验收，
然后完成完整103点及新图。新binary不能重标/混入gate11前35点；原输入/aliases复用。
本轮为接收三点、离线平台验收及真实procfs故障修正/build的progress，无外部阻塞。

## 故障前观察：35/103点，进入No-image Image-Go

同一Python3318838/pane3318832、tmux `streams8_formal_gate11_matrix_r1`实际核实
live；elapsed2:20:43时stdout35个完成点，完整命令无`--only`。SplitSnap与Sabre
各17点全部收齐，第35点No-image AES也完成；当前No-image Image-Go。
本轮新接收Sabre Product Catalog/Shipping和No-image AES，三点Matrix/metrics、
identity/shutdown/resources、WS插页/实际payload对账通过，共2,100calls已收。
详见[接收记录](../experiments/currentbase17-streams8/results/20260910-r1-gate11/RECEPTION_AUDIT.md)。

本轮新增本地离线`platform_evidence.py`及聚合接入：正式图发布前逐点复核三节点
before/after，不再仅依赖当时runner曾通过；不查询远端、不修改部署probe。
35点/210份实际保存的节点快照已全检通过。12项定向tests通过，覆盖缺失/CPU及
网络漂移/先后顺序/聚合拒绝，以及既有Mean/负收益/静态边界；无新codec实机冒烟。
新`gate11-analysis-platform-r1.json/.tar.gz`包含此前style/METHODS/FINDINGS、平台
离线检查/测试及更新REPRODUCE，直接叠加source-overlay-r3；tar compare rc0。
不再用旧style/methods/findings增量覆盖新版；runtime/runner/deployed toolset/frozen
均未改。未生成不完整正式图包；全103点与逐项发布审计仍待完成，goal保持active。
上一轮文档任务为设计边界确认；本goal turn是新增三点接收和离线平台验收实现progress。

## 最近一次实施观察（非本轮核实）：32/103点

以下“本轮”指产生该记录的上一实施阶段，不代表这次文档任务执行了这些动作。

同一 tmux `streams8_formal_gate11_matrix_r1` / Python3318838（pane3318832）
已重新核实 live，完整命令无`--only`；elapsed2:08:48时已32个POINT_COMPLETE，
进入Sabre Product Catalog。没有重启controller或重放请求。
Sabre第21–32点（包括本轮新增Fib-Py/Currency/Email）
已按最终Matrix/metrics函数接收复核：
各60calls/480流提前输出，full WS插页数及每调用payload匹配frozen，60次VM/UFFD
释放及整unit65所属VMM缺席。十二点共720calls，累计32点/1,920calls。
详见 [本轮接收记录](../experiments/currentbase17-streams8/results/20260910-r1-gate11/RECEPTION_AUDIT.md)。
没有新codec/负向冒烟或全库SHA；runtime/runner/toolset未改。
前32点before/after平台记录全部追加复核通过，192份节点记录符合既定CPU/
SMT/Turbo/10G-IFB口径，无重复backend TBF；不代表未来远端状态。
上一轮新增本地findings.py，正式聚合在全103点验收后自动导出FINDINGS.md：同轮
六系统raw-mean比较、逐workload有符号改善/退化，以及容量/payload减少率和边界。
静态预览不输出性能结论；原图/统计公式及103点guard未改。8项定向tests通过。
最新分析增量`gate11-analysis-findings-r1.json/.tar.gz`包含先前style/METHODS，
直接叠加source-overlay-r3，supersedes analysis-methods-r1；tar compare rc0。
不向运行节点部署此后处理增量，尚未生成完整正式图包。
本轮另补[REPRODUCE.md](../experiments/currentbase17-streams8/REPRODUCE.md)，并实际
在独立临时目录按base→source-r3→analysis-findings-r1恢复；七个关键脚本/plan
cmp一致，聚合CLI --help退出0。证据`P/reproduction-source-check-r1.json`；只证明
源码层叠/入口，不是bit-identical构建或全图重绘。没有构建/部署/换binary。
上一goal turn为三点接收和findings生成/归档progress；本轮为Fib-Py/Currency/Email
验收、平台覆盖扩至32点，以及复现指南/源码恢复实查progress。
后续仍查同一controller，补完所有系统/AES local并输出完整正式图包；不将局部
E2E差异全归因于八流布局（实施稿§0.5）。下方为历史快照，不覆盖这条最新观察。

## 最近一次实施记录：20点通过（本轮未更新进度）

- 本地 tmux `streams8_formal_gate11_matrix_r1` / Python3318838（pane3318832）
  本轮重新确认存活，完整命令无`--only`；根`results/20260910-r1-gate11/`、
  attempt `controller-matrix-r1`。stdout已20个POINT_COMPLETE，进入Sabre VideoProc。
  不从本文推断之后的实时进度；后续仍先查同一controller/job。
- 全17个SplitSnap remote归档点用最终 `Matrix.validate`、`parse_metrics`、`medians`、
  `quality_summary` 再次接收检查通过，另核对POINT_COMPLETE/config/version、
  shutdown/resources身份。每点60calls、整unit65所属VMM缺席，WS/private页数
  与冻结inventory匹配；累计1,020calls/510measured，8,160流全部提前输出，
  每点65所属VMM缺席（全17点共1,105个所属实例记录）。
  不裁掉长尾：例如Fib-Go后30次额外UFFD事件median6.5/max611，完整保留。
- Product Catalog/Shipping为第16/17点，也已验收完整收尾；measured E2E median
  分别46.8157195/41.040435ms，WS content12.827/13.0625ms。未重复发请求。
- Sabre17份冻结输入均核对full WS、原页数/index一致、streams8-v1/八流。
  AES实际配置security=full、coalescing=true、9562端口、gate11 binary；其完整
  WS为4,027页/16,494,592raw bytes/5,337,902压缩bytes，不混SplitSnap的1,516
  private页/1,076,057压缩bytes。Sabre AES窗口现已完整收集、追加验收：60calls、
  480流提前输出、320,274,120payload bytes（60×5,337,902）、60次VM/UFFD释放，
  整unit65个VMM缺席；全部调用插入4,027页。后30次E2E median51.38104ms、
  WS content21.3895ms、嵌套WS decode21.375ms。总计18点/1,080calls；未重跑。
- Sabre Image-Go/Image-Py随后完整收齐且追加验收：各60calls/480流提前输出、
  60次VM/UFFD释放、整unit65VMM缺席；完整WS分别21,239/32,330页，每调用压缩
  payload为8,590,812/26,670,173bytes，与frozen及实际读取精确匹配。后30次
  E2E median734.131546/2177.880111ms，WS content60.2105/86.237ms。
  当前共20点/1,200calls；保留原数据，不把局部差异外推全suite。
  新统计脚本/图的完整验收仍等103点，未发布残缺正式包或对未跑baseline宣称收益。
- 本轮只改本地aggregate的Figure10排版（纵轴标题两行、刻度0/.5/1/1.5），
  新静态预览`results/20260910-r1-static-r2/`已生成并目检；两份数值CSV与r1
  cmp均rc0。聚合/来源5tests通过，未创建残缺正式性能包。
  `gate11-analysis-style-r1.json/.tar.gz`为分析层增量，叠加runtime source-overlay-r3；
  新增量tar compare rc0，r3排除文档/aggregate后compare也rc0。测量binary/runner/
  已部署toolset/frozen均未改，无新负向/codec冒烟或全库SHA。外部依赖已有
  loader源码、worker binary/config及分析环境记录，后续与最终包一并核对。
- 本轮新增METHODS.md，按当前footprint/metrics/aggregate源码明确实际对象大小
  与shared/global WS容量模型、Figure11 native重分类与Execution残差、median/
  Mean、SHA/并行wall、资源和WS quality口径。aggregate随结果自动复制并链接它。
  `gate11-analysis-methods-r1.json/.tar.gz`是最新分析增量，已包含style修正，
  直接叠加source-overlay-r3；不能再用旧style包覆盖它。static-r3成功导出、文档
  cmp相同、两份CSV与static-r2 cmp均rc0、5项聚合/来源tests通过，archive compare
  rc0。runtime/runner/toolset/frozen未改；正式全103点guard未放宽。
- 原worker relay4180610、containerd4180024、demux4180059在上一轮实机确认存续，
  本轮未重启或更换它们；本地controller本轮已核实存活（约80分钟）。
- 上一轮归类为progress（18点验收/图排版修正）；本轮完成Sabre Image-Go/Image-Py
  验收及方法文档导出/源码归档。下一动作是同一任务继续Sabre及其余
  系统/AES local，完整103点与正式图包仍未齐，goal active。

## 历史：实施细节落文档，未接续实验

入口：[实施稿 §0](SPLITSNAP_SORT_IMPLEMENTATION.md#0-本次确认与下一次-goal-待办)。
§0.4 按现有代码补清了八长 Range 的接口、应用层首读/首输出计时、嵌套 wall/SHA、
clean 与非 clean 的内存差异，以及 CLI/统计/外部依赖边界。并非新增实现或实验。
一个 payload 八条独立长流，原页序/PFN不变；不循环1-MiB GET，不任意切旧压缩流，
也不宣称已经消除 host staging 或必然加速。

已有候选、gate11 局部证据/r3 归档与 frozen accounting 保留，不照旧缺项重复实施。
外部 worker 的两个 modified=true 构建依赖精确保留的 binary，不单靠嵌入 commit。
获准后先查历史 `streams8_formal_gate11_matrix_r1` / `controller-matrix-r1` 同一
任务的真实状态再决定下一步，不能按此文的8/103等旧数值直接重跑。完整交付还须
逐点核对采集、整 unit 退出、资源、版本及新图；本轮没有进行这项验收。

## 历史实施快照：gate11 矩阵、统计接收与外部依赖

- 本轮重新确认本地 tmux `streams8_formal_gate11_matrix_r1` 和 Python3318838
  存活（pane3318832），完整命令未使用 `--only`。根
  `results/20260910-r1-gate11/`，attempt `controller-matrix-r1`。已完整收齐8/103：
  SplitSnap AES-Go、Image-Go、Image-Py、VideoProc、VideoAn、AES-Py、AES-NJS、Auth-Go；
  Auth-NJS运行中。Auth-Go unit已实机确认inactive/MainPID0，由同一controller推进。
  以后仍先查同一controller/job，不按此快照直接重启或改运行参数。
- 八个已收点均额外用最终统计同一函数重新验收：POINT_COMPLETE、config、shutdown/
  resources身份，60slot调用/decoder/VM/UFFD，以及metrics/WS页数对账全部通过。
  每点480流均首输出早于末次读入，整unit各65所属VMM全退出；保留page-fault长尾。
  这不是最终全103点的聚合，也没有创建残缺正式图包。
- `snapshotting-final-focused.log` 已确认 focused race PASS（1.434s）；codec六组
  与Python40历史日志已定位。旧ctriface整包API编译失败仍披露，不称全仓测试通过。
- 本地r3源码tar compare排除文档后rc0，运行代码未改；原worker relay4180610
  经SSH确认存续。AES-Py专属unit也曾确认active（PID52144），随后由原controller收尾。
- 补充 `provenance/20260910-r1/external-direct-invoker.json`：loader/helper二进制
  与本地、原currentbase17输入receipt同SHA，源码为vSwarm a096cc4且tracked clean；
  小源码archive与Go build-info已归档。未重建/替换loader，未把此事后观察写入frozen。
  loader无/usr/local/go/bin/go不影响测量；构建信息从同SHA本地binary读取。
- 新 [逐项验收索引](SPLITSNAP_SORT_ACCEPTANCE.md) 汇总实施稿全部有效要求、代码/
  证据和缺项。完整矩阵、剩余baseline/AES local、正式新图/结果包仍待验收；goal active。

## 历史：文档复核与后续入口

入口：[实施稿 §0](SPLITSNAP_SORT_IMPLEMENTATION.md#0-本次确认与下一次-goal-待办)。
编码、MinIO 长 Range、mmap/cache、八 C1 decoder 和 UFFD 边界已按本地源码核对。
本地已有 gate11 negative 的七 VMM shutdown 记录；r3 release/源码/工具包已存在，
工具包含 late-cancel 与 accounting。runner 已冻结实际静态报告，正式聚合使用该
快照；Python40 tests/OK 是已有日志，不是本轮新测试。旧 API 整包编译失败仍保留。
这些覆盖下方历史“缺 shutdown/缺完整包/统计仍读可变来源”的待办，不重复实施。

历史矩阵入口为 `results/20260910-r1-gate11/controller-matrix-r1/` 与
`streams8_formal_gate11_matrix_r1`；本轮不核实当前活动状态或更新完成数。下次获准
先查同一任务，接收已有材料后补真实缺口；不要按历史 1/103 重跑已完成窗口。
最终必须有完整 103 点、同版来源、退出/资源记录及新 Figure9–13/WS quality 验收。

## 历史实施观察：gate11 退出证明与正式输入冻结

- 上一轮文档审计属于 progress：发现工具包落后与统计来源未冻结，改变本轮下一步。
- 只读实机核实 gate11 negative unit 已 not-found/inactive/MainPID0；补收其
  shutdown.json 和 relay-shutdown.log，七个所属 VMM 全缺席。原 relay PID4180610
  仍运行。本轮没有重跑缺 WS/晚期取消，也没有再次停止该 unit。
- 同版本负向、晚期取消、整个池退出的前置验证已通过；102 行静态 payload 的
  source/endpoint/layout/实际对象大小也已与 inventory 对账通过。
- 新增 accounting_inputs.py：将实际 footprint/payload 报告嵌入新 frozen.json；
  正式聚合只读该快照，缺失即失败，static-only 预览才读 provenance。保留 native
  粒度与 Figure9–13 原统计公式，没有重扫 corpus 或新增全库 SHA。
- Python 40 tests 已通过，relay binary 未改变；既有 Go 文件级 race 通过及旧
  ctriface 整包 API 编译失败仍如实保留。完整源码 overlay-r3 和 toolset-r3 已归档，
  三节点 tar compare 均 rc0；所有 tracked runtime 改动均在 overlay 文件清单中。
  `provenance/20260910-r1/gate11-release.json` 绑定基线、源码包、工具包和 binary SHA，
  本地/worker 一致；preflight 每 controller 检查一次 binary，不做全库 SHA。
- 正式 preflight 已通过：六项准备/统计任务 rc0；三台 c6620 各28物理核、
  2.1GHz、SMT/Turbo off；worker IFB10G/400ms 参数，backend 无重复 TBF。
  backend 剩余约26GiB，未清理旧 corpus。记录见 gate11 结果根 platform-preflight.json。
- 完整 matrix 已启动并核实本地 tmux/PID 存活：`streams8_formal_gate11_matrix_r1`，
  pane3318832 / Python3318838；结果根 `results/20260910-r1-gate11/`，attempt
  `controller-matrix-r1`。已完整收齐1/103：SplitSnap AES remote；现在进入 Image-Go，
  新 unit 已启动，controller仍存活。先查同一任务，不重启窗口，不修改正在执行的工具/relay。
- 首点60/60成功，480/480流有末次读入前首输出，64,563,420 payload bytes；
  60/60 VM stop与UFFD释放，最终整unit的65个VMM（含未使用池）全部缺席。
  shutdown/relay-shutdown/resources均已收齐。measured slots30–59：E2E median
  42.2017435ms、WS content10.5245ms、nested WS decode9.628ms；不是全矩阵收益。
  原始与summary入口在 gate11结果根 points/splitsnap-zstd3--aes-go-45000-45450--remote。
- frozen.json 已包含全部103点、30+30 slots、完整静态报告和 release 身份。
  终点仍为完整矩阵、Figure9–13/WS质量和逐项发布验收，不能以“已启动”算完成。

以下是更早文档审计及实施记录；本轮执行边界以本文顶部为准。

## 历史文档复核与后续入口

入口：[实施冻结稿 §14](SPLITSNAP_SORT_IMPLEMENTATION.md#14-当前冻结八长流方案不变补齐生命周期与版本边界)。
八长 Range / 八 C1 decoder 的设计及后续 17×6 + AES all-local 范围不变。

- 本地 gate9 历史记录是三个点（AES remote/local、Image-Go）局部验收；
  matrix-r1 后在 Image-Py 启动检查退出。此处不报告当前远端进度或在线状态。
- gate9 缺 WS 负向测试已有 `INVALID.md`：客户端正确失败，但删除 shim 后
  预启动 VMM 仍存活，最终 idle 超时；不是单纯退出竞态。该测试自己的六个 VMM
  有历史精确退出记录，不把人工善后当作测试通过。
- gate10 的缺 WS `negative-validation.json` 已 accepted，`shutdown.json` 记录
  该 unit 六个 VMM 全退出。gate11 的缺 WS 与 LoadSnapshot 后请求取消也各有
  本地 accepted/idle 记录；后者明确 `performance_sample=false`。这是已有证据，
  本轮未重跑 validator/测试/调用。
- 最新 gate11 候选已调整为先清池/停 VMM 再释放网络与 backing；关闭 refill，
  保留失败成员并上传错误。launcher 保存整 unit VM ownership/退出证明，runner/
  aggregate 已接入；不再称这些全部“待实现”。idle 仍不能替代 unused 池退出。
- 本地未找到 `aes-negative-gate11/shutdown.json`；缺 WS 与晚期取消共用同一
  unit。下次先查 `provenance/20260910-r1/aes-negative-gate11.json` 指向的
  job/unit 和 `logs/shutdown-before.json` / `shutdown.json`，优先补收，不重复发请求。
  本轮没有核实停机是否完成，也不由缺文件推断仍运行或失败。
- 历史 `old-owned-vmm-retirement-gate11.json` 记录 55 个已结束测试遗留 VMM
  精确退出。本轮没有清理；不能将旧 INVALID 转正，也不能直接将旧/新差异全部
  归因于八流。当前线上资源状态未检查。
- 本地 gate11 文件级 Go race / Python 37 tests 历史日志通过；`ctriface` 整包
  focused tests 因旧测试 StartVM/LoadSnapshot API 不匹配而编译失败，须区分报告。
- 当前 runner 默认 gate11；但已存 `gate11-toolset.tar.gz` 的 run_matrix 仍缺
  晚期取消/整池退出前置条件，也没有 `cancel_after_load.py`。完整 gate11 源码包
  尚未找到；build-info 和旧工具包不能当成最终可复现版本。获准后核对部署再补包。
- 新 binary 用新 frozen 结果根，不混 gate9 三点。原 raw/PFN、inventory/aliases
  和静态报告可按身份复用；aggregate 当前仍从固定 provenance 读取静态报告，
  要与 frozen run 再固定/对账。完整 103 点、新 Figure11/12 和最终包尚未验收；
  BW/C3/r12 仍单列，1-RPS 主实验不自动成为隔离带宽点。

以下是原实施记录，保留 provenance，不构成本轮继续执行的授权。

## 历史：gate9 接续时的观察

- 已核实旧 matrix-r1 controller rc1、tmux/PID 缺席；Image-Go 已有完整验收和
  资源记录，因此当前已收 3/103。Image-Py 在 launcher 的 plain bind preflight
  因 EADDRINUSE 失败，未创建 unit/发请求；loader 精确 job 不存在。
- 只读 worker 检查：8090/8091 无 listener/TCP socket、无 streams8 unit，旧 relay
  4180610 存续。TIME_WAIT 是源码+定向复现支持的推断，不是当时抓到的 socket。
- launcher probe 改为 SO_REUSEADDR + listen，仍拒绝真实监听者，不用 REUSEPORT。
  本地 5 个 launcher tests 通过，源码包 gate9-launcher-r5.tar.gz；只部署 worker
  启动器，relay-streams8-gate9 binary/测量路径不变。旧失败日志完整保留。
- 当前先补 gate9 缺 WS 负向验证：专属 worker PID 34878/negative-gate9 unit
  已 ready，loader job 为 gates/aes-negative-gate9-job，复用已有无 payload fixture。
  完成收集/验收/idle 后仅停止这个测试 unit，再接续同一 gate9 正式矩阵。
- 前一目标轮分类为 progress：文档审计明确了版本/归档/统计缺口。本轮有实际
  launcher 修正、定向测试/部署及负向检查进展；不以历史 active 状态当任务存活。

## 历史：前一轮文档复核

详细冻结与接续清单见 [实施稿 §13](SPLITSNAP_SORT_IMPLEMENTATION.md#13-当前文档冻结设计已有候选与后续验收分开)。

- 设计不变：一个大 payload，页对齐八独立长流；八长 Range/八 C1 decoder，
  输出 host mmap 后进入原 UFFD。不重排页、不删 SHA、不改变 Chunks/Pages。
- 本地源码已含 gate9 有序失败清理、payload-job preflight、窗口后打包收集、
  全 103 点先验收再输出、WS quality 和资源记录。修正旧“尚未实现”的条目。
- 只读核对 gate9 AES remote/local 的 POINT_COMPLETE、validation 和 local
  fetch stats：各 60-call 已有局部验收记录，local store disabled/0 reads。
  本轮未重跑 validator，不把局部记录当成全矩阵完成或当前远端 ready。
- gate9 历史 Python 32 tests / OK 日志和完整源码 overlay-r2 已存在；后者已含
  metrics/aggregate。此次文档变化未重打包进旧 overlay，不覆盖其归档 provenance。
- 后续重点：旧负向 gate 不能证明 gate9 失败清理；POINT_COMPLETE 后仍需完整
  资源归档；服务生命周期 CPU/内存峰值不是每次解压 CPU/RSS；统计报告身份须与
  frozen run 一致；Figure15 当前接口是 CSV，不能声称已生成新图。
- 历史接续线索：本地 tmux `streams8_formal_gate9_matrix_r1`，结果
  `results/20260910-r1-gate9/` / controller-matrix-r1。**本轮未查当前是否运行、
  最新完成数或远端状态。** 下次获准先查同一任务，不重复启动或重放窗口。
- 后续主范围仍是 17×6 + matched AES all-local 及新统计/图，BW/C3/r12 单列；
  不能从此前“goal active”记录自动恢复执行。

## 历史实施记录：gate9 失败清理收口与完整正式矩阵

以下时间点的 2/103、PID 和“当前”仅是上一轮记录，不是本轮重新核实的在线状态。

- gate8 AES remote 已验收：60/60 调用、60 个 VM stop/UFFD 释放；480 条流均有
  输入未读完即输出的记录，64,563,420 payload bytes。worker 的该独立 unit 已
  `not-found/inactive/MainPID=0`，旧 relay PID 4180610 存续；loader window rc0，
  本地 controller 已结束。不是仍待观察，也不重复启动 gate8。
- gate9 将 LoadSnapshot 失败清理合为一次有序流程：取消未连接 listener、移除
  shim、核对该 VM 进程缺席及 UFFD 释放、最后 Free network/map/IO。失败则保留
  状态并返回错误，有限等待只约束 teardown；正常 fetch/解压路径不变。
- 定向文件级 Go race 通过；含 native/local parser、Mean、WS 事件质量、打包
  收集与资源记录的 Python 32 tests 通过（`python-tests-final.log`）；relay-streams8-gate9
  已构建部署。不是全仓测试声明。
- preflight 已加入 payload-job 结束检查。aggregate 在建立正式输出目录前验收
  全部 103 点，防止静态图先出现被误当成正式包完成。静态报告继续复用。
- gate9 AES remote/local 均已完整验收：各 60 calls/60 stops/60 UFFD release，
  相同 64,563,420 payload bytes；local store disabled 且 0 requests/bytes。
  remote 后 30 次 E2E median 39.842842 ms、WS content 9.897 ms；全 WS 4,027 页、
  private 1,516 页与 inventory 一致。额外 UFFD 事件 median 4、max 585，保留长尾。
  不混入 gate8 性能样本，不预先作全矩阵收益结论。
- remote controller-aes-r1 的逐文件 SCP 过慢，仅中断已核实的收集子进程；r1 rc1
  与说明保留。controller-aes-r2 用压缩 tar 流收回同一已结束窗口，未重放调用；
  r2 和 controller-local-r1 都 rc0。新 collector 在计时窗口之后工作。
- **完整矩阵已启动，当前验收 2/103。** 本地 tmux
  `streams8_formal_gate9_matrix_r1`，pane PID 3293311、Python 3293317 已核实存活。
  根 `results/20260910-r1-gate9/`，controller-matrix-r1；已跳过 AES remote，
  当前进入 `splitsnap-zstd3:image-rotate-go-11:remote`。先检查同一任务，不重复启动。
  AES local 已有完成记录，完整 runner 到该点会重新审计后跳过。
- 新 `relay-resources.json` 归档 systemd 的服务生命周期 CPU/内存峰值，包含启动/
  warmup/测量/收尾/收集等待，且不含其它 cgroup 的 guest/containerd；不是单次
  解压 CPU time 或精确 RSS。两个 AES 点均已补收该记录，无额外函数调用。
- 聚合新增 Figure15 runtime 质量 CSV，与固定全 WS/private 页数对账，给出
  whole-WS 比率和明确命名的旧 private+recipe 比率；事件不是 unique remote pages。
  Figure13 仍全部排除 recipe，不混淆两种口径。
- 源码完整 overlay 和列表在 provenance 的 `gate9-source-complete-overlay-r2.tar.gz`
  / `gate9-source-files-r2.txt`，相对基线 3f18475f；build-info 固定 gate9 binary。
  之前增量包保留。操作入口见 [实验 README](../experiments/currentbase17-streams8/README.md)。
- 剩余：其余 101 个正式点、当前失败清理版本的必要负向实机确认（复用已有缺 WS
  工具，不再跑五函数 codec）、完整 Figure11/12/WS 质量与最终逐项验收；BW/C3/r12
  仍单列。没有完整 goal completion 证据，保持 active。

详细设计见 [实施冻结稿](SPLITSNAP_SORT_IMPLEMENTATION.md)。以下是前一文档审计的
历史记录。候选/证据以顶部本地复核为准；当前执行边界仍是仅文档。

## 历史：前一轮文档核对结论

- 方案仍为一个大 payload、八条独立长 Zstd 流、八长 Range、八 C1 decoder，
  原始页序不变，输出到 host mmap 后进入原 UFFD；没有改成 1-MiB 循环下载。
- 本地已有 codec/runtime、独立 launcher、103 点 runner/probes/validator，以及
  gate8 的精确 VM 终态/UFFD 释放确认和失败时结构化诊断。已有
  `verification/local-20260910-gate8/` 的文件级 Go race PASS、Python 23 tests OK；
  本轮未重跑，不称全 package/全仓通过，不按历史“尚未实现”重复工作。
- [AES local gate3 验收](../experiments/currentbase17-streams8/verification/remote-20260910-r1/aes-local-gate3/validation.json)
  已本地保存：一次 gate6 功能调用、1,076,057 bytes；最终 snapshot store disabled、
  0 requests/bytes、无活动 VM/UFFD/异步任务。旧 local gate2 INVALID 不变；
  gate3 不是新版 60-call 正式 all-local 验收。
- [footprint.json](../experiments/currentbase17-streams8/provenance/20260910-r1/footprint.json)
  已本地保存（生成于 2026-09-10T09:27:11Z），明确八流布局与 17-workload 跨 tier
  union；不再是“仅构建/等待运行”。[payload.json](../experiments/currentbase17-streams8/provenance/20260910-r1/payload.json)
  也已完整保存：同 run/layout、102 行。既有
  [static-only 输出](../experiments/currentbase17-streams8/results/20260910-r1-static-r1/README.md)
  已含 Figure9/10/13 的 CSV、PNG/PDF；无需重复采集，不代表新 latency 完成。
- `gate8-sources.tar.gz` / `gate8-toolset.tar.gz` / build-info 已存在；源码包含
  payload 工具，但不含后加的 `metrics.py` / `aggregate.py`。两者候选已存在，
  下次补版本索引、少量 native/local/归一化检查及完整包发布验收，不从零重写。
- 正式 AES r2 尝试的本地证据现已收回：`invocations.tsv` 60/60 exit0/reply_ok/
  native、window rc0，但 controller rc1。relay 有 60 条 `forcefully terminated VM`
  StopVM 错误；末态观察 requests/UFFD/异步任务为零，vmMap 仍保留 60 项，idle
  超时，无 POINT_COMPLETE。**这是诊断记录，不是有效正式性能点。**
- `gate7-terminal-observation.json` 补有历史进程观察：vmMap=60，但对应 restored
  VM 进程为空、其余活动计数零。支持 gate7 残留记录的诊断，不将其自动转正；
  gate8 已有修正候选，不能继续把“任意 StopVM error 即返回”称作当前实现。
- [gate8 AES remote](../experiments/currentbase17-streams8/results/20260910-r1-gate8/)
  已有 started/ready/controller argv 等本地记录。本轮没有验收终态或监测进程，
  **既不判断仍运行，也不判断失败；获准后先查同一 job/unit，不重复发窗口。**
- 剩余源码边界：LoadSnapshot 失败 defer 仍先 Free 再处理 shim/UFFD，晚期失败
  不由正常 StopSingleVM 修正自动覆盖；preflight 未列后加的 payload-job；
  aggregate 先出静态图再审正式点，部分输出不能当完整发布。这些只记录待办，
  本轮不修实现、不测试、不采数。
- 完整 103 点、同版本 60-call AES all-local、新 Figure11/12 和复现包仍未验收。
  沿用 measured median / mean-of-raw-values 口径，整窗 60 次 counter 不冒充后
  30 次；只在下一次 goal 获准后接续，旧结果、服务和代码保持原样。

## 历史：上一轮 goal 实施进展

以下 PID/任务状态只表示当时记录，本轮未复核；其中 footprint/local gate3/runner
现状以顶部本地证据为准。

- 核实 loader 的 AES local gate3 job `rc=0`；worker gate6 PID 16571 存活且 idle，
  旧 relay PID 4180610 仍运行。已收回 gate3 原始调用/relay/counters/最终 runtime
  状态及 local-aliases receipt。local 功能 validator 通过：一次同表示 AES、
  1,076,057 compressed bytes、store disabled、0 reads，UFFD/VM/异步任务均收尾。
- §11.6 的早退 StopVM 改为不继承请求取消；StopVM RPC 失败不再报成功或释放 VM
  记录/清理 backing cache。UFFD listener 支持取消尚未连接的 accept，失败 restore
  在 shim 回收后等待 handler/release 完成；已接受连接仍属于 VM 生命周期。
- 新增 identity-keyed `RESTORE_COMPONENTS`，避免多调用时散行 metric 混配；
  formal validator 进一步对账 VM/组件/invocation/StopVM/UFFD，拒绝旧宽泛 stop-error 例外。
- `run_matrix.py`、`point_worker.py`、`platform_probe.py` 已新增：103 点完整编排、
  固定 30+30 slot、独立配置/版本、readiness/idle/最终 counters、可接续但不自动重放
  未知状态请求窗口。正在做本地测试与部署接通，尚未产生正式矩阵样本。
- Backend 原 inventory/aliases jobs 均 `rc=0`；磁盘剩余约 26 GiB，未清理旧数据。
  新静态 accounting job `/users/Liquidz/streams8/20260910-r1/footprint-job/`，tmux
  `streams8_footprint_r1`、PID 3931702 已核实运行；待其结束再采正式性能。
  只读新 corpus，输出新 `footprint.json`，不与旧图混用。

## 开工前本地复核（当时没有新测试或新实验）

- 分支仍为 `likun/splitsnap-sort`，有已修改源码、未跟踪工具和 binaries，原样保留。
- 已有候选包括 codec/manager、隔离 launcher、prepare/viewcopy、inventory、aliases、
  localcache/local_aliases、negativews、gate/validator 和 runtime probe。relay
  使用 `r.Context()`，正常异步清理/refill 已脱离请求取消；正式 validator 已读取
  inventory/materialized aliases 并核对 slot/每流 manifest。strict-local 已要求
  `storage_disabled=true` 和零计数，窗口末必须无活动 VM/UFFD/异步任务。
  修正较早“这些尚无实现”的说法，但不由源码存在推断已验收/部署到同一版本。
- 已有两份小/大 WS 功能记录，见下表；本轮只读 validation/result，没有重新运行
  validator、构建、测试或发函数请求。二者不是正式 latency 或全套生命周期验收。
- 已读本地 [准备报告归档](../experiments/currentbase17-streams8/provenance/20260910-r1/preparation-reports.tar.gz)：
  `COPIES_TRANSCODES_COMPLETE.json` 记录 18 copies、9 transcode jobs；
  `ORACLE_VIEWS_COMPLETE.json` 记录 512/2048/3072 三档报告。这修正此前“view
  尚待执行”的历史条目，但不代表当前远端对象/readiness，也不代表 aliases 完成。
- 本地已找到 [actual-layout.json](../experiments/currentbase17-streams8/provenance/20260910-r1/actual-layout.json)：
  run `20260910-r1`，18 stores、六系统各 17 行；68 行 coalesced streams8、
  34 行 native not-applicable。另有 [aliases-complete.json](../experiments/currentbase17-streams8/provenance/20260910-r1/aliases-complete.json)：
  同 run/layout、`materialized=true`、5,100 aliases、tag `streams8-20260910-r1`。
  报告是既有落盘证据，本轮只读，不代表当前远端对象或剩余空间已重新复核。
- [negative gate2](../experiments/currentbase17-streams8/verification/remote-20260910-r1/aes-negative-gate2/negative-validation.json)
  已有独立 `accepted=true` 验收：缺必需 WS、客户端失败、函数/UFFD 均未开始、
  shim 已释放。旧 wrapper 的 `expected_failure=false` 是只匹配 body 的限制，
  不覆盖独立日志验收结论；本轮未重跑。此记录对应 gate5，仅覆盖 UFFD 启动前。
- [local gate2 INVALID](../experiments/currentbase17-streams8/verification/remote-20260910-r1/aes-local-gate2/INVALID.md)
  记录了“调用成功、0 reads，但 StopVM/refill 被取消”，不能算 all-local/release
  通过。`local_aliases.py` 已有 60-alias 工具，`aes-local-gate3.json` 指向 gate6；
  本地尚未找到 gate3 最终结果和 local-aliases 完成 receipt。不据此推断远端任务
  失败/仍运行，不重启；下次获准后先检查已有任务及结果再接续。
- 剩余重点：正式 outer runner、同版本 all-local/释放验收、非 WS component 与
  VM 身份关联、窗口末收尾后采 counters；精确 alias/slot/manifest 对账不再是空白。
  新源码风险包括早退 StopVM 仍继承取消，以及 UFFD listener 启动后 CreateVM
  失败可能无人解除 Accept；这是待处理/验证项，不是本轮实测故障（见 §11.6）。
  Figure9/10 必须使用跨三档首次出现顺序的 footprint 入口，不能取 tier oracle
  报告中的 hash-order 压缩值，也不能把三个 tier 的压缩结果直接相加。
- 下列 tmux/unit/目录仅供之后获准实施时定位已有任务；本轮未检查它们是否仍运行，
  不重复启动，也未暂停或停止其中任何服务。

| 已落盘功能证据 | 成功调用 | 八流 payload bytes | 首输出早于末次读入 |
| --- | ---: | ---: | ---: |
| [AES gate2](../experiments/currentbase17-streams8/verification/remote-20260910-r1/aes-gate2/validation.json) | 1 | 1,076,057 | 8/8 streams |
| [Image-Py large gate1](../experiments/currentbase17-streams8/verification/remote-20260910-r1/imagepy-large-gate1/validation.json) | 1 | 20,099,896 | 8/8 streams |

以上均带 restore/UFFD/invocation 计数；本轮不由一条成功记录推断失败释放或
all-local 已通过。后续使用时核对记录对应 binary，不能自动覆盖更新后的候选。

## 历史实施交接：准备 controller 与独立 runtime 启动

本节保留当时进度；其中“仍运行/尚待执行”已被顶部本地归档证据部分覆盖，
不是当前远端状态。后续获准后先读已有 job/rc/report，不按本节直接重复启动。

- 原只读扫描 PID 3923301 已退出，scan.rc=0；18 stores、6,991,029 objects、
  18,869,222,994 logical bytes。`capacity/` 读取旧 WS manifest 后另算新 WS 与 aliases，
  报告保存在 `provenance/20260910-r1/capacity.json`。完整 aliases 仍待实际压缩尺寸
  和物理空间检查，不按旧压缩率当新结果；至少预留最大一批 60 aliases。
- 新 `prepare.py` 已在 backend tmux `streams8_prepare_r1` 运行，任务目录
  `/users/Liquidz/streams8/20260910-r1/prepare-job/`；先看进程/日志/rc，不重复启动。
  18 个新 MinIO 已启动（9561–9566、9571–9576、9581–9586），数据位于原实验 XFS
  挂载下的独立 `streams8/20260910-r1/`，原 946x–948x 与其它服务保持运行。
- 三档 SplitSnap 17 份 private WS 复制/转码已经完成；controller 继续其余原生
  stores/Sabre/No-image。它只发布 COPIES_TRANSCODES_COMPLETE，不冒充 oracle、
  aliases 或正式矩阵完成。`viewcopy/` 已构建，待准备结束后复用新 partial 表示。
- `point_config.py` 完整 102 行与 AES all-local 映射测试通过（连同 launcher 5 tests）。
  relay 固定新监听 8090、网络名称 sstr、IP 段 172.30/172.31，独立 cache/scratch。
- 第一次新 runtime 启动（AES gate1）发现独立部署的两个实际缺口：旧 cache HTTP
  仍抢 8081；WalkDir 不跟随根 images 软链接，导致 image inventory 为零。尚未发
  函数请求，已仅停止该任务 unit，旧 relay PID 4180610 未停止。
- 新候选增加 `wsCacheEndpoint`（独立 8091）并解析 images symlink；focused race
  `TestIsolatedCacheLoadsSymlinkedImages` 与 102 配置/launcher tests 通过。
  `relay-streams8-gate2` 已构建；精确候选源码 tar 在 `provenance/20260910-r1/`。
  函数恢复 gate 尚待执行/审计，不能把服务启动称为真实恢复完成。

以下此前记录保留 provenance；与顶部本地复核冲突时以顶部为准。尚无完整新正式
性能结果；oracle/inventory/aliases/negative gate2 已有本地归档，当前远端存续、
同版本 all-local、runner 和新统计/图仍待验收。

## 已有候选代码（此前本地验证，不代表系统验收）

- `snapshotting/zstdstreams`：EncodeTo 直接写 writer，页对齐近似八等分；每条长流
  一次 Range reader、一个 concurrency=1 decoder；输出直接写各自 mmap 区间。
  独立版本/后缀、CRC/SHA/长度验证、8-MiB window 和每流轻量时间/字节统计已实现。
- manager/MinIO/relay/converter：默认 `streams8-v1`，旧 frame-size CLI 移除；
  WS 配置拒绝 W10/旧 frame size。Context 从 LoadSnapshot 传入 payload 与新 WS
  metadata 读取；不再整包下载 fallback，不再 private 出错读取 full WS。
- relay 正常异步清理、shim refill/失败归还 shim 已使用不继承请求取消的 context。
  `runtime-state` / `runtime_probe.py` 观测本任务 requests/VM/cleanup/refill/
  compressed-cache 写入/UFFD handler；formal validator 要求窗口末 idle。现有
  `lifecycle_test.go` 等是候选测试源码，本轮未运行；早退取消缺口见冻结稿 §11.6。
- 正式 compressed/coalesced WS 的必要 pages/content/index 错误传至 LoadSnapshot，
  错误路径释放已取得的 buffers。保持 recording/raw/base 的既有允许缺 WS 语义。
- 压缩缓存仍异步保存：临时文件/完整发布、revision 锁、generation 防止过期发布、
  已打开 fd 与 eviction 分离；新后缀进入 inventory/容量和 fetch 分类。
- `cmd/materialize_full_dedup_oracle_ws`：从原 dirty oracle worktree 按文件移入
  main.go/test，保留 canonical/view 语义，更新新布局/报告字段。正式 view validation
  直接解码新 private 表示后验证 canonical/PFN，不再强制多存一份 raw private 文件。
- `experiments/currentbase17-streams8/footprint`：representative17 统计的新版本，
  shared/global union 与 WS 均采用新编码器；tier port bases 必须显式传 JSON。
- 同目录 `transcode`：明确以旧 `.zstd.frames/.zstd.json` 为只读离线输入，冻结 PFN
  顺序，对新目标转码/本地验证后上传 payload、原 index、最后 manifest。不重录 WS，
  不覆盖既存目标对象。已有逐 revision 成功 journal、批次锁与 resume，已复制的
  相同 index 保留；无成功记录的部分对象不能隐式当成已完成。已有 prepare controller，
  但全系统 inventory/aliases/measurement controller 尚未贯通。
- relay/网络配置已有 `snapshotsDir`、`snapshotsScratchDir`、`networkNamePrefix`；
  `launcher.py` 有独立 unit/cache/scratch/endpoint 的候选；`run-direct-window.sh`
  已参数化完整 relay 地址。已有一次独立 AES 功能记录，不代表完整 outer runner、
  全矩阵统计与 all-local 接通。
- `corpus_plan.py` 和 `config/20260910-r1/` 已有三档 18 stores/102 配置计划；
  `copycorpus`、`start-corpora.py`、`run-corpus-scan.sh` 已存在。计划、脚本和 binary
  存在不表示真实新 corpus、aliases 或布局 inventory 已完成。

## 此前本地证据（本轮未重跑）

[focused race 测试日志](../experiments/currentbase17-streams8/verification/local-20260910/tests.log)。
该次测试覆盖新 codec、manager、native chunk/provenance 回归、MinIO counter 分类、
Full Dedup materializer helper 和转码 PFN parser；全部通过。包含四 policy remote/local、
整页/小/空边界、输入未收完先有输出、失败取消、无 private→full fallback、缺 index、
损坏不发布、并发发布/读取/淘汰、旧 token 不复活。无 race 报告。
日志里测试夹具缺 images 目录的旧初始化提示不是远端故障；这些单测不证明真实 UFFD。

构建入口：

```bash
go build -o bin/relay-streams8 ./cmd/relay
go build -o bin/snapshot-converter-streams8 ./cmd/snapshot_converter
go build -o bin/materialize-full-dedup-streams8 ./cmd/materialize_full_dedup_oracle_ws
go build -o bin/transcode-ws-streams8 ./experiments/currentbase17-streams8/transcode
go build -o bin/footprint-streams8 ./experiments/currentbase17-streams8/footprint
```

本地 Go 1.26.7，库仍 klauspost/compress v1.17.11。代码未 commit/push；bin 是未跟踪
输出，不要用仅基线 commit 描述新 binary。部署前保存精确 source patch/版本信息。

## 此前远端记录（本轮未重新连接，不作为当前 readiness）

- `Liquidz@er020.utah.cloudlab.us` SSH -A 成功，worker 仍存续。
- `Liquidz@er069.utah.cloudlab.us` SSH -A 成功；docker ps 显示原 9460–9466、
  9470–9476、9480–9486 三档 template/systems/oracle 服务及数据库/registry 仍运行。
- 仅做 hostname/uptime/df/docker ps，没有部署新 binary/corpus，没有停止原服务。
  未完成 loader、CPU/NIC、真实 WS/PFN 可读取、数据挂载剩余空间、服务占用的全套核实。

以上是早一轮只读检查记录。随后交接还提及 backend 的独立准备目录
`/users/Liquidz/streams8/20260910-r1/`、只读 corpus 清单 helper 和 tmux
`streams8_corpus_scan_r1`；**本轮不连接、监测或重启它**，也不把此前“运行中”
当成现在状态。之后获准实施时先检查已有 scan.log/scan.rc/source-inventory.json，
确认是否完成或仍运行，不能重复启动。helper/配置传输不等于新 relay 部署或新
corpus/正式测量完成；当前远端状态未复核。

## 尚未完成，待用户之后开 goal 执行

1. 先读 AES r2 失败包，核对已有 worker unit/loader job 和精确 binary；不重放旧
   窗口，不重置未知状态 counters。gate6 local gate3 已有成功记录，不再从零寻找。
2. 核实固定依赖的 StopVM 终态，修正已确认强制终止后的剩余资源释放及 verifier；
   补失败结构化状态采集。只做最少同版本定向检查，不重复已有 codec 冒烟。
3. 复用三档准备、Full Dedup view、102 行 inventory、5,100 aliases 和 footprint
   报告；下次仅核对存续/剩余空间与工具版本，不重新录制 WS、不重物化完整 corpus。
4. 接通已有 payload collector 的实际报告以及 Figure11/12 parser，保存聚合 CSV/
   新 Figure9–13 和复现说明。静态扫描在测量窗口之外，recipe 不计入 payload。
5. 修正版新 attempt 完成 17×6 + matched AES-Go all-local，每点 30 warmup +
   30 measured；原失败和退化均保留，旧图不覆盖。当前没有通过完整验收的新性能包。

BW sweep 和 C3/r12 trace 是主计划单列依赖，不由本地代码测试自动变成已迁移。
本轮没有启动或监测实验，不能据此断言远端当前没有其它任务。

## 较早文档审计条目（最新细化见冻结稿 §11）

- 独立 cache/scratch CLI 和 launcher 已有候选，修正此前“仍固定、未实现”的
  描述；默认仍指向旧路径，不能省略参数。还需实际网络地址段隔离和服务共存验证。
- 旧 restart/all-local helper 会停止通用服务和切换公共 cache；应派生独立 launcher，
  不能直接运行。监听、invoker、counter、readiness 的 8080 要一起参数化。
- 旧 helper 对 native chunk 压缩也传已移除的 zstdFrameSize；六行都需清理参数。
- 正式 Full Dedup 是 `security=partial` 的 oracle transfer-view，不使用候选的
  monolithic security=full-dedup 路径；AES all-local 对齐 SplitSnap，不是 Sabre。
- transcode 已有逐 revision report/resume，但未提交对象仍需明确 attempt 边界；
  corpus/view/aliases 的总完成标记尚未接通。102 行 inventory 不要求 native 行有八流。
- interval>1s 的旧 driver 仍采用绝对时刻表；前台无重叠不等于固定空闲间隔，
  保留主实验定义，隔离/BW 单独验实际间隔。
- 磁盘预算需额外计入新八流 WS 和 5,100 个 alias revisions；只读源清单排除了 WS
  payload，不能拿该清单大小直接证明整个准备过程空间足够。
- 正式 runner 是 worker ifb0 入站限速，拒绝 backend 重复 TBF；应核实新端口也被
  覆盖。每对象八路不等于节点全局八路，重叠请求会叠加资源使用。
