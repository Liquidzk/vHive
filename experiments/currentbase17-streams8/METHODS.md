# 八长流结果：统计、计时与复现口径

本文件解释生成方法，不是运行状态或完成证明。结果目录的 README 区分 static-only
预览与完整正式包；静态图存在不表示 103 个性能点已完成。

## 输入与系统范围

正式范围为 17 workloads × 6 systems 的 102 个 cold-remote 点，以及匹配
SplitSnap AES-Go 的一个 all-local 点。每点 60 calls，slots 0–29 warmup，30–59
measured。原始记录保留全部调用、失败和长尾；不按数值挑选后 30 个“好样本”。
主矩阵采用 absolute 1 RPS，调用及异步清理可能重叠，不是隔离带宽实验。

三档 VM 内存为 512/2048/3072 MiB；沿用 currentbase17 同一份 raw WS、PFN、
源 snapshot 与请求定义，转码不重录、不排序、不重新分类。三台 c6620 的平台
检查记录 2.1 GHz、28 物理核、SMT/Turbo off，worker ingress IFB 10 Gbit/s；
具体 run 仍以其 platform-preflight 和逐点 before/after 为证，不能据本文推断
任意时间或新环境也有相同设置。

正式聚合还用 `platform_evidence.py` 离线复核每点三节点的 before/after 文件：
角色/CPU型号、28个在线CPU及policy、2.1GHz/performance、SMT/Turbo off、worker
10Gbit/400ms IFB match-all redirect、backend无重复TBF，以及同节点采集先后顺序。
完整103点应有618份节点观测，缺失或不符在输出前报错；只读保存的记录，不查询或
改变正在运行的节点。这不是连续监控证明，也不把400ms队列上限解释成固定网络RTT。

| 图例 | 正式输入与语义 |
| --- | --- |
| Chunks | 原生 128-KiB compressed chunks，无 coalesced WS 长流 |
| Pages | 原生 4-KiB compressed pages，无 coalesced WS 长流 |
| Sabre（ws-zstd3） | full security，每函数完整 WS，八长流 |
| SplitSnap-（no-image-zstd3） | no-image-sharing，对应 private WS 八长流，image 留在 private |
| SplitSnap | partial，private WS 八长流，原 base/rootfs/image 共享路径不变 |
| Full Dedup | runtime 使用 partial-format 的预生成 transfer view；容量用 canonical global union 的 unsafe oracle |
| All-local | 与 SplitSnap AES remote 相同输入/表示，仅变更驻留；必须 storage disabled、零远端读取，仍需解压 |

Full Dedup 的 view assembly 与临时占用不计入 oracle；不是声称现有 runtime 已经
完成通用的按需全局 gather。相同 view 的 payload 可与 SplitSnap 完全相等，不制造差距。
这里也不是 C3/r12 的十节点 trace，既不乘节点数，也不按 trace 的调用频率加权。

## Figure9/10：压缩内容容量，而非磁盘/内存实测总占用

来源是 run `frozen.json` 内的 `accounting.footprint`。`footprint/main.go` 对三档
来源做跨端点对象身份 union，维持确定性的 source/PFN 首次出现顺序；不能相加
三个独立压缩 union 报告，更不能使用 hash-order 的另一个 oracle 报告代替。

- Snapshot 部分按实际所需 native 压缩对象大小统计，按各 policy 的身份去重。
- Sabre WS 和两种 SplitSnap private WS 使用实际八流 payload 大小。
- 共享 WS 的容量按所需共享页的跨档 union，用同一八流编码器重新估算；Full
  Dedup WS 是全局唯一内容的理想 coalesced 编码。它们是明确的容量模型，不是
  对 worker 实际驻留内存、guest RSS 或所有 backend 文件的直接测量。
- 不计 recipe、info、snap、index、manifest 和 MinIO 内部开销；原始快照分母
  仍包含未修改完整内存文件中的零页。运行中仍会使用这些 metadata，未删除它们。

设 `F=raw_full_snapshot_bytes`、`S_s=snapshot_bytes`、
`W_s=working_set_storage_bytes`、`C_s=active_cache_bytes`：

| 输出 | 公式 |
| --- | --- |
| Figure9 蓝段、橙段、总量 | `S_s/F`、`W_s/F`、`(S_s+W_s)/F` |
| Figure10 | `C_s/C_Sabre`；Chunks 超轴部分用其真实倍率标注，不改变原值 |

不能把 Figure10 称为某一 trace 时刻的 cache utilization，也不能把它与包含
metadata/OS cache 的 resident memory 混用。原始字段在 `figure9-10-footprint.csv`。

## Figure11/12：measured medians 与组件边界

`metrics.py` 以 request/revision → VM → UFFD 身份对应日志，避免并发日志串线。
`components-per-call.csv` 保留 60 slots；`measured-medians.csv` 对每个组件分别取
后 30 次的 median。它不是所有均值，也不表示所有组件来自同一次 median invocation。

Figure11 按既有 component 口径计算，令各字母代表该系统 AES measured median：

- `E`：relay E2E；`D/U/P/W`：download、get_uffd、get_ws_pages、get_ws_content；
- `L`：load_vmm；`I`：WS pre-insert；`H`：page_fault_handler；
- `X=max(0,I-I_Sabre_remote)` 仅用于 Chunks/Pages，其它系统为 0；
- Fetch + Decompression：`D+U+P+W+X`；Restoration：`max(0,L-X)`；
- Page Faults：`max(0,H-I)`；Execution：`max(0,E-Fetch-Restoration-PageFaults)`。

`X` 是将 native pre-insertion 相对 Sabre 的额外成本重新归类；蓝段不是纯网络
读取，也不是只等于 private WS 的 GET。Execution 是上述口径下的残差，不是独立
采集的 guest CPU 时间。聚合器检查总和与 E2E 一致，出现矛盾报错，不静默改 E2E。
`preinsert_reclassified` 与各段保存在 `figure11-components.csv`。

Figure12 令 `T_s,w` 为每 workload 后 30 次 relay E2E median，柱高为
`T_s,w/T_Pages,w`。Mean 是 `mean_w(T_s,w)/mean_w(T_Pages,w)`，不是各比例的平均。
改善率须写清比较对象和分母；不能由单个 AES 点外推整个 benchmark suite。

## Figure13：压缩 payload 字节

来源是 frozen `accounting.payload` 的完整 102 行和精确对象尺寸清单。native 行
选择所需 chunk/page 对象，coalesced 行选择指定 WS payload；不纳入 recipe、
index、manifest，也不把 runtime 的所有 metadata/lazy GET 总量冒充同一指标。

令 `B_s,w=compressed_payload_bytes`，柱高为 `B_s,w/B_Chunks,w`，Mean 为
`mean_w(B_s,w)/mean_w(B_Chunks,w)`。这是内容 payload 口径，不是链路抓包字节、
TCP 重传、HTTP headers 或 SDK attempts；它也不是解压前 raw WS 页数比。

## 八流与资源日志不能混加

一个 payload 最多八条独立长 Zstd 流，每流一个长 Range/C1 decoder，直接写
host mmap 不相交区间。全部解码和每流 SHA 完成才进入既有 UFFD；不是
direct-to-guest/零拷贝，也不是无 SHA 的独立解压 microbenchmark。

`ZSTD_WS_DECODE.elapsed_us` 覆盖 manifest、mmap、GET、解码、校验与同步记账，
嵌套在外部 WS content wall 内；不能加两次。每流首读/读完/首输出是应用 reader/
decoder 返回时刻，不是网卡到包时间。流数、range opens 与实际 HTTP attempts
不同；并行流耗时或 SHA wall 不能相加后从 outer wall 中扣除求“纯网络/无 SHA”。
非 clean miss 仍有完整 compressed-cache buffer/异步发布；clean remote 没有
该副本。八路窗口上限不等于整个进程 RSS 上限。

`relay-lifetime-resources.csv` 是专属 systemd service 全生命周期 CPU time 和
其四舍五入的 memory peak，包括启动、warmup、测量、收尾与采集等待；不代表
每次 decode CPU/RSS，也不涵盖其它 cgroup 的 guest、containerd 或 MinIO。
其 `platform_observations_verified` 列记录每点已核对的六份平台快照，与资源
统计的覆盖范围分开，不把平台通过当作每次解压CPU/内存测量。

## WS quality 与完整发布

`figure15-ws-quality.csv` 中额外 UFFD events=`handled-1`，不是唯一远端缺页数。
全 60 次插页量对账，统计取后 30 次并保留 max/长尾；whole_ws_pct 的分母是完整
WS 页数，不含 recipe。明确命名的 legacy_private_plus_recipe_pct 仅保留旧口径，
与 Figure13 payload 指标无关。本次交付是质量 CSV，不是假装已有新 Figure15 曲线。

正式聚合先重新验收全部 103 点，检查 calls/layout/身份、VM/UFFD、shutdown、
资源和 frozen 静态来源，再建立新的输出目录。POINT_COMPLETE 先于 unit 退出
材料，单独存在不证明包已收齐；采集中断只补收同一窗口，不重放请求。旧 INVALID
保留，不混入 gate11，也不隐去失败/退化说明。全仓测试通过不能由局部 tests 推断。

源码基础以选定run的 `frozen.release` 为准：gate12是 `gate12-release.json`
指定的base + 完整source/toolset-r1，已含当前统计/平台检查；不要叠加旧gate11
分析包覆盖它。历史gate11则是其release指定的source/toolset-r3及另列的分析增量。
运行后补充文档不重写frozen或更换测量binary；两版本的性能点不能混成同一矩阵。
外部 invoker、worker modified binaries/config、原始 corpus 和分析库版本也有
独立来源记录，不包含在一个 relay 源码包里。完整包还须人工逐项核对来源、图和
数值解释；脚本退出 0 不单独构成目标完成。
