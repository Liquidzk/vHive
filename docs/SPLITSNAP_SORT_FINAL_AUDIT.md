# 八长流系统迁移：最终交付审计

日期：2026-09-11。分支 `likun/splitsnap-sort`；运行版本 gate12。
本表覆盖 [实施稿](SPLITSNAP_SORT_IMPLEMENTATION.md) §0–14 的有效契约，合并重复
要求；旧章节中已经被 §0 覆盖的“尚未实现”和历史重跑命令不重新执行。
**完整103点及正式图包已核验；最后的文档归档核对另记录于F/FINAL_DELIVERY.json。**

路径约定：`E=experiments/currentbase17-streams8`，`P=E/provenance/20260910-r1`，
`R=E/results/20260910-r1-gate12`，`F=E/results/20260910-r1-gate12-figures-r1`。
这些路径相对仓库根；代码、原始结果、源码包、图包共同组成交付，不只是一张图。

## 1. 实现与正确性

| 有效要求 | 权威实现及检查材料 | 判定与边界 |
| --- | --- | --- |
| §1–2：一个 payload、最多八个独立长流；4-KiB 页边界、余页均分、空/不足八页 | `snapshotting/zstdstreams/streams.go::EncodeTo/Manifest.Validate`；`E/verification/local-20260910/tests.log` 中 RoundTrip 0/1/7/8/9/19/3000、无效输入/manifest 测试 | 已核实；不任意切割已有单压缩流、不补零、不重排原页 |
| §2：固定 v1.17.11、Zstd-3、8-MiB window、CRC、每流 SHA、C1 | go.mod、EncodeTo/decode 选项、ConfigureCompression、relay/converter CLI；TestStreamsRejectOldConfiguration | 已核实；WS 默认 streams8-v1，只接受八路，不保留旧小帧参数的运行时回退；`-j16` 另指 UFFD 并发 |
| §1–3、5：每流一次长 Range、增量解码、无整包 ReadAll/1-MiB 队列/末尾拼接 | `storage/minio_storage.go::OpenObjectRange` 使用 inclusive end；decode 每流独立 reader/C1 decoder，写 dst extent；TestOutputBeforeInputTail | 已核实；正常路径每对象最多八路，应用 reader 次数不是全部 HTTP attempts；仍是 host staging 后交 UFFD |
| §3、5：失败/父取消关闭各 reader，所有写者退出后释放 | decode 的派生 context、AfterFunc/once Close、errgroup.Wait；manager 的 release；CorruptionAndTruncation/FailureCancelsAndClosesEveryRange/ParentCancellation | 已核实定向覆盖；不宣称所有旧 shared raw GET 都已改为可取消 |
| §3：压缩缓存收费、local 读取、成功后原子发布、淘汰后不复活 | `manager_streams.go` 的 openCachedStreams/publishStreamCache、epoch/锁；registry 文件清单；remote/local/corrupt/expired/concurrent tests | 已核实；clean remote 不保留全压缩副本，非 clean 仍 Tee 到完整压缩 buffer，local 同一 fd 的 SectionReader；未重写缓存策略 |
| §3：必需 private payload/index 失败不能回退 full/lazy；source/learning 保留 | manager policy/context 与 LoadSnapshot 失败分支；MissingPrivateDoesNotFallbackToFull/RequiresPrivateIndex；gate12 negative-validation | 已核实；真实缺 WS gate 客户端失败、零函数调用/零 UFFD 安装，不以普通 HTTP500 代替原因证明 |
| §11–14：请求取消不取消善后、UFFD listener/VM/池退出顺序 | ctriface iface/orch/vm_termination、relay cleanup、UFFD 生命周期；gate12 missing-WS、cancel-after-load、shared unit shutdown | 已核实同版缺 WS、真实 LoadSnapshot 后取消及整个专属池七个 VMM 缺席；后者不是 CreateVM 内每个位置的故障注入，也非任意负载下停机的穷尽证明 |
| §12.6–14：forced-stop 不能只匹配错误字符串放行；procfs 退出竞态 | vm_termination.go 的精确 VM 身份、进程缺席、UFFD 完成及 Free 顺序；`P/gate12-procfs-fix-local.json` 的文件级 race/launcher 测试 | 已核实；ESRCH/ENOENT 并发退出可忽略，权限/I/O 错误仍失败；旧失败点不转正 |

核心测试源码与 gate12 的适用性由 `P/gate12-core-test-source-continuity-r1.json`
说明：八个 codec/cache/storage 实现和测试文件与已有 focused race 证据版本逐字节
相同；不把旧 gate11 性能点改标 gate12。gate12 生命周期变更有自己的测试与实机证据。
`E/verification/local-20260910-gate11/ctriface-focused-tests.log` 仍记录旧测试 API
签名不匹配的整包编译失败；**没有“全仓测试全部通过”的声明**，不为此修改无关 API。

## 2. 系统、输入与实验编排

| 有效要求 | 权威证据 | 判定与边界 |
| --- | --- | --- |
| §4、7–8：完整六系统映射、三档、raw/PFN/分类不变 | `R/frozen.json` 的 plan/inventory/accounting；transcode、materializer/viewcopy、preparation reports | 已核实完整102行：68 coalesced、34 native；Sabre full WS，No-image/SplitSnap 对应 private WS，Full Dedup partial-format oracle view；Chunks128KiB/Pages4KiB 保留 |
| §4、7.1、11.4：Full Dedup canonical 与 transfer view 不混用 | canonical/PFN 验证、三档 view receipts、footprint 跨档 first-occurrence union | 已核实；runtime view 预生成，组装/临时副本不计 oracle；17项 payload 与 SplitSnap 相等是该定义的实际结果，不宣称通用在线 global gather 或实测延迟严格下界 |
| §8、10–11：新对象/manifest 发布、按身份续作、容量与 aliases | `P/preparation-reports.tar.gz`、actual-layout/aliases-complete、R/frozen；prepare/transcode journal/aliases 实现 | 已核实18 stores、102行、5100物化 aliases；原始 WS 不重录，旧压缩布局只作为离线转码输入；未复制旧 COMPLETE 或反复重建 corpus |
| §7.2、10.2：独立 cache/scratch/endpoint/网络，不接管原服务 | plan/point_config/launcher、逐点 config/launch/shutdown；原环境来源及精确 retirement 记录 | 新8090/8091、sstr 名称及172.30/31地址、新956x/957x/958x stores；不改 HOME、不全局清 cache、不运行旧通用 kill/restart helper |
| §7.3、10.4、11.5：readiness、准备互斥、固定平台、cadence与样本 | runner/prerequisites、两个 endpoint probes、invocations.tsv/calls、platform-before/after | 同一绝对1RPS、每点60slots（前30warmup/后30measured）；准备/静态采集六个任务均结束再测，平台由逐点保存快照核对，不冒充连续监测或孤立 fetch |
| §9、12–14：完整17×6 remote + matched AES all-local | `R/controller-matrix-r1`、每点调用/VM/UFFD/final/shutdown/resources；F/DELIVERY_CHECKS.json | 已通过：controller rc0、103点/6180calls/3090measured、每点65所属VMM缺席、618份平台快照；不是只检查POINT_COMPLETE |
| §5、11.5：真实小/大 MinIO→UFFD与同表示 all-local 零读取 | 同版 AES/Image-Py remote 正式点；matched AES local 原始配置、60calls、final fetch stats、local-preparation | 已通过：local与remote同4,027全WS页/1,516private页、1,076,057bytes八流payload和60aliases；store禁用/0reads，E2E median33.2669555ms。零读取仅针对snapshot/WS/lazy store |

失败 gate7/9/11 与历史局部结果保留在原目录。gate11 前35点不混入 gate12，第36点
仍为 INVALID；诊断资源退役不使失败样本变有效。新一轮未为了性能退化重录 WS、
改变 cadence、去 SHA 或重放正式窗口。

## 3. 统计、图和复现

| 有效要求 | 最终产物/公式 | 核验结果 |
| --- | --- | --- |
| §4、11.4：Figure9/10 跨三档全局首次出现 union，原分母 | F/figure9-10-footprint.csv、Figure9/10 PNG/PDF；frozen accounting | 已通过六行逐字段对账；两图目检，Chunks真实倍率4.45×超轴标注，未改原值 |
| §12.4：Figure11 keyed medians、native reclassification、嵌套计时不重复 | F/figure11-components.csv、Figure11 PNG/PDF；七根AES柱和逐段和等于E2E | 已通过七行组件和/对应E2E及图形核对，local解压非零但remote reads为零 |
| §12.4：Figure12/13 Mean 为原始标量均值之比 | F/figure12-e2e.csv、figure13-payload.csv、两图PNG/PDF；各17项+Mean | 已通过各90行的原始值、分母和比值独立计算；两图目检无裁切 |
| §4、5：Figure13 不计 recipe/index/manifest；计数口径分开 | frozen payload 完整102行；最终METHODS | 静态输入已核实；不以60次runtime全部读取量代替内容payload，不把counter除以30 |
| §12–14：WS quality 完整17项、全WS/private页数、长尾保留 | F/figure15-ws-quality.csv 与原始日志、已接收17行子表比较 | 已从原始日志重建并逐字段匹配子表；510 measured共10次>100额外事件、最大607；events不是unique remote pages |
| §13–14：103点原始/资源和退化说明 | F/components-per-call.csv、measured-medians.csv、relay-lifetime-resources.csv、FINDINGS/METHODS | 已通过6180调用、103中位数与全部receipts、103原始资源消息对账；service-lifetime CPU/rounded peak非每次decoder CPU/RSS；三项相对Sabre退化保留 |
| §13–14：同版源码、binary/toolset和外部依赖 | P/gate12-release.json、source-complete-overlay-r1/toolset-r1、build-info、deployment-source-check、external-* | 已核实运行版本来源；105非Markdown成员与归档一致。两个external worker binary modified=true，复现保留精确binary，不假称仅commit足以重建 |
| §13–14：实际源码恢复、完整结果重绘、新目录不覆盖 | P/gate12-reproduction-source-check-r1.json、gate12-full-aggregation-r1.json、E/REPRODUCE.md | 已从真实恢复的base+source-r1执行完整aggregate，退出0；新F中8份CSV、五图PNG/PDF及说明齐全。105运行/分析源码成员不变，不构建部署或发请求；文档增量另包 |
| §0、6、14.5：归因/范围边界 | 最终FINDINGS/METHODS、本审计 | BW、C3/r12、IAA不在本次终点；Figure4分类来源不变但未重绘。新版还含配套修正，不能把旧/新全部E2E差异解释成纯八流收益 |

## 4. 最终证据摘要与边界

- `P/gate12-full-aggregation-r1.json`：真正从恢复源码运行完整aggregate，退出0，
  Python3.12.3/Matplotlib3.10.8/NumPy2.4.3与已记录分析环境相同。
- `F/DELIVERY_CHECKS.json`：103点/6180calls/3090measured、618份平台快照、全部
  scalar/slot/WS页数/launch/退出/资源、两张归一化图与组件和核对通过。
- 五张PNG人工目检：图例有框、柱/标签/刻度可见、无裁切；五个PDF均可解析且各1页。
- `P/gate12-final-original-services-r1.json`：原三个服务PID/命令行与历史记录相同，
  原uvmns1–58仍在；只读观察，不停止或清理服务。
- `P/gate12-delivery-docs-r1.tar.gz`及其来源清单：运行后文档层，包含本最终审计和
  复现指南，不改运行/分析代码。包内容逐文件核对记录于`F/FINAL_DELIVERY.json`。

同轮SplitSnap相对Sabre的E2E raw-mean减少3.216%，相对Chunks减少39.070%；
14/17 workload比Sabre快，VideoProc/VideoAn/Fib-NJS较慢，原值保留。
相对Sabre，内容storage减少39.368%、cache模型减少38.180%、mean payload减少55.805%。
这些是本轮系统比较，不是八流相对旧小帧的独立收益或无校验codec速度。
全仓旧API测试编译失败、Full Dedup oracle边界、absolute1RPS可能重叠、service资源
计时边界均已披露；不据此宣称BW/C3/r12迁移或所有失败/停机时序穷尽覆盖。
冻结源码包、release、frozen输入和旧图不覆盖；工作树保持未提交，未自动commit/push。
