# SplitSnap 主实验 17-workload：系统级八长流迁移计划

日期：2026-09-10。

2026-09-11交付更新：gate12完整103点、6,180calls及新Figure9–13/CSV已核验，
见 [最终审计](SPLITSNAP_SORT_FINAL_AUDIT.md)和
[新图包](../experiments/currentbase17-streams8/results/20260910-r1-gate12-figures-r1/README.md)。
下方为原范围/执行计划，不能依据历史候选或待办文字重启已完成矩阵。

**范围修正版：覆盖完整 currentbase17 主实验，而非五 workload 的 private-WS
codec 小测试。** 此版取代上一版“先五个小测试，再考虑推广六系统”的交付范围。
目标是在真正的转换、MinIO 获取、缓存、VM restore、六系统实验和统计链路中
用八条长流替换应用层 1-MiB 小帧；大 WS 的独立流数不随对象大小增长。

**当前：用户 goal continuation 恢复完整实施。** 本轮已重新核对 gate12 三节点
工具/worker二进制与源码身份，精确清理已退役失败实例的网络，推进同版验证。
原服务、输入与历史结果保留；设计与完整103点/新图范围不变，见实施稿§0及
[验收索引](SPLITSNAP_SORT_ACCEPTANCE.md)。具体任务和验收进度以STATUS为准，
不由局部负向通过推断完整矩阵完成，也不重标 gate11 旧点。

设计已确定：一个 payload 内八条独立长流，八条完整长 Range GET + 八个 C1
decoder；不采用循环 1-MiB Range 队列，不重排原页序。具体实现边界和本次复核见
[实施细节冻结稿 §0](SPLITSNAP_SORT_IMPLEMENTATION.md#0-本次确认与下一次-goal-待办)，
已有证据见 [状态](SPLITSNAP_SORT_STATUS.md)。gate11 的整 unit 退出本地记录、
完整源码/toolset-r3、frozen accounting 候选已存在，不照历史缺项从头重做。
下方原基线问题与迁移步骤是设计/验收清单，不表示所有项当前均未实现。
后续获准先核实同一任务真实状态、来源与版本，再补缺口并验收完整 103 点和新图；
实时完成数以 STATUS 所指同一 controller 和归档材料为准，不以局部结果声称全系统完成。

## 1. 分支与范围

- 分支：`likun/splitsnap-sort`。
- 工作树：`/home/liquid/invitro-related/.dist/vhive-splitsnap-sort`。
- 起点：正式单函数工作树 `vhive-figure4-snapshot-alignment` 的
  `3f18475f5f5665848d1e4f7745d9c92e75b1a7ec`。
- 原工作树、IAA 未提交实现、trace Figure7/8 分支及原 corpus 均保持不变。
- 分支已在此前创建；本轮不切分支、不 commit/push，本文目前也是未提交文件。

交付目标是 **17 workload × 6 系统 = 102 个主实验配置，加 AES-Go all-local**，
所有采用 coalesced WS 的正式路径都使用八长流。不是只让五份 private WS
通过 standalone benchmark，也不是仅修改一个 decoder 库。

| 系统 | 必须覆盖的主路径 | 保持不变的语义 |
| --- | --- | --- |
| Chunks | 纳入整套构建/runner/回归测量；无 coalesced WS 多流入口 | 128-KiB native chunk 对象、压缩和按需获取 |
| Pages | 纳入整套构建/runner/回归测量；无 coalesced WS 多流入口 | 4-KiB native page 对象、压缩和按需获取 |
| Sabre（代码 ws） | 17 份完整 coalesced WS 转码、下载和解压换八长流 | 每函数的完整 WS 和隔离边界 |
| SplitSnap−（no-image） | 17 份 policy 对应 private/image WS 大对象换八长流 | base/rootfs 共享，image 仍归 private |
| SplitSnap | 17 份 private WS 大对象换八长流 | base/rootfs/image 共享，敏感内容隔离 |
| Full Dedup | 17 份正式 coalesced transfer view 与 materializer 一起适配 | canonical global page store、offline-oracle 计时边界 |
| AES-Go all-local | 新版系统同一压缩表示在完整本地缓存中恢复 | 0 remote GET，不等于免解压 |

Chunks/Pages 的 128-KiB/4-KiB 粒度是 baseline 语义，不是要取消的 1-MiB WS
应用层压缩分帧；把它们也合成八个大对象会改变 baseline，故不这样做。
不修改共享/私有判定、recipe、WS PFN 集合、native chunk 大小、UFFD 安装语义。
`sort` 暂仅为分支名：**不自行增加按内容/hash/PFN 重排页面的优化**；
保留输入字节顺序及原 index 对应关系。

17-workload 清单和请求以正式 workloads.json / direct_requests.json 为准：
AES、Auth、Fibonacci 各 Go/Python/NodeJS 共 9 项，加 Currency、Email、Product
Catalog、Shipping 共 13 项，均 512 MiB；Image-Go、Image-Py、VideoProc 各 2 GiB，
VideoAn 为 3 GiB。复用同一代 base、一次 source 和五次独立 restore-learning
得到的输入，优先只重新编码表示，不重新生成 VM/工作集。

十节点 C3 trace 使用 700 logical functions / 44 profiles，不能当作这 17 个单函数
配置。其运行代码同样受 WS codec 变更影响，纳入下方发布依赖检查；本轮评估更新
不自动授权重跑十节点 trace。r12 scope-frame cache 的特殊性不阻塞 17-workload
主实验迁移，也不能拿旧 trace 图冒充新实现结果。

## 2. 原正式基线实现核对（不是未提交候选的当前状态）

| 正式代码 | 当前行为 |
| --- | --- |
| `snapshotting/zstdstream/frames.go::Encode` | 每约 1 MiB raw 调用 EncodeAll，产生独立 frame，串接到一个 payload；逐帧记录位置、大小和 SHA。 |
| 同文件 `Decode` | worker 按帧创建 reader/decoder，每个 decoder concurrency=1，直接写 destination 对应 raw 区间。 |
| `storage/minio_storage.go::OpenObjectRange` | SetRange 后返回增量 reader，不先完整下载 range。 |
| `snapshotting/manager.go::getWorkingSetPathManaged` | 分配一个 raw mmap；Range reader 直接进入 Decode，非 clean 路径额外 tee 压缩缓存。 |
| `ctriface/iface.go` | 正式调用 Managed WS 接口，输出释放交给恢复生命周期。 |

“假流”不是精确技术描述：当前 MinIO 路径已经能 fetch/decode 重叠，
也没有解压后将所有小帧再拼接的步骤。需要替换的是 **大量小独立帧、逐帧
GET/decoder 创建和调度**，不是从零引入流式读取。
仅改后缀、增大 zstdFrameSize 或将 Fetchers 改成 8，不算完成新实现。

通用 DownloadObject 虽然也可并行下载，但收齐 buffer 才返回；新解压路径
不能调用它来冒充边下载边解压。

## 3. 推荐实现

### 离线编码：八条独立长流，一个 payload

1. 使用冻结的 WS raw 和 index，不重录 WS、不重新分类。
2. 按页数近似等分八段，4-KiB 对齐，顺序不变；每段页数差不超过一页。
   不足八页只生成非空流，空 WS 无流，不生成填充页。
3. 每段一个独立 Zstd writer，增量 Write、最后一次 Close；level 3、
   encoder concurrency=1、内置 checksum 开启。不每 1 MiB Reset/Flush。
4. 八条流顺序写入同一文件，直接写 file/writer，不先保存八个完整 byte slice
   再复制拼接。这是离线对象布局，不是 restore 前的聚合屏障。
5. 索引记录各流 raw offset/length、compressed offset/length、完整性字段，
   以及版本、总大小、stream count。Range GET 必须知道边界，**仍需要索引**。

建议独立格式 `zstd-streams-v1`，如 `.zstd.streams` / `.zstd.streams.json`，
实现时冻结字段。旧 frames-v1 保留；新模式不隐式回退到旧格式或 raw。
首次编码顺序执行即可，不额外引入离线压缩并行化。

### 在线恢复：每流一条端到端流水线

每条流 i：`Range GET(offset_i, length_i) → decoder_i → raw mmap 对应区间`。

- 每个至少八页的 coalesced WS 对象固定八个 ranges/readers/decoders；每 decoder
  内部 concurrency=1，生命周期覆盖整条流，不随小块重建。
- 每路独立 MinIO Object/reader，不在同一可变 Object 上并发 Seek。
- Range 从独立流起点开始，正常无重试时每流一个 GET；首版不在每流内再嵌套
  八个小 GET，避免 64 路并发及新的聚合队列。
- 一个父 context；任一路失败取消其它 GET、关闭 reader/decoder，等所有写入
  退出后才释放 mmap，避免取消后仍写已释放地址。
- 不在解压前 ReadAll 完整 payload，不跨流按序等待，不创建最终 concat buffer。
  各路直接写自己的 raw 区间，全部完成并验证后返回给现有 UFFD 路径。
- 输出仍是宿主侧 mmap staging，不是直接写 guest，也没有取消 UFFDIO_COPY。
  首版不引入“解压一页立即安装 guest”的新调度。
- local cache-hit 使用同布局 SectionReader + decoder；all-local 仍需解压，
  但应为 0 远端 GET。

**格式限制：**普通 Zstd frame 内的块可依赖此前内容，不能把已有单流的压缩
字节任意切成八个 ranges，再交给八个独立 decoder。独立性必须在编码时建立。
八条长流在标准格式上仍各自是一个 frame；取消的是应用层固定 1-MiB 小帧布局，
不是删除 Zstd 标准 frame。依据：
[Zstd 格式规范](https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#definitions)、
[固定版本 klauspost 文档](https://github.com/klauspost/compress/blob/v1.17.11/zstd/README.md)。

### 缓存、完整性及兼容

- 保留 compressed-cache 计费、淘汰和访问登记。新后缀进入 cache inventory，
  也进入 payload/metadata fetch counter，不能漏计或归到 other。
- 缓存 tee 的压缩副本不是 decoder 的前置聚合屏障。首版不承诺消除所有缓存
  buffer，不把相关成本藏到计时外。
- 八流全部成功后才发布缓存；临时/部分 payload 不能成为 cache hit，失败不登记。
  并发发布和释放做针对性验证，不展开无关缓存重构。
- 首轮 A/B 保持两边同等校验：内置内容 checksum、长度/EOF、每帧/流 raw SHA。
  不重复加整份或逐页 SHA 扫描；输入冻结/编码时核对一次即可。
  若以后删除恢复 SHA，应独立比较，不能同时改校验和布局后归因给多流。
- Zstd 内置内容 checksum 为截断 XXH64，不是密码学认证；不替代 provenance
  内容身份或安全判定。recipe/metadata 在实际恢复中仍然需要。
- 保留资源/window 限制并记录 RSS；长流的 decoder 历史窗口可能比小帧更大。
- 首版只保证 MinIO RangeObjectStorage 和本地文件路径，不另写整对象 fallback。

## 4. 从原始输入到新论文数据：完整流程

1. **冻结 17-workload 输入与六配置矩阵。** 读取
   `snapshare-eval/results/figure9-16/20260902-currentbase17-r1_c6620_systems_tiered_r1/`
   下的 workloads/direct_requests/matrix/corpora；核对三内存档位、PFNs、policy 和
   原始 WS/index。缺 raw 时可离线解码已验证的旧对象恢复同一字节，不默认重录。
2. **编译 converter/relay/materializer/分析工具。** Full Dedup materializer 在
   `.dist/vhive-full-dedup-oracle`，新分支起点并不包含它；必须移入必要源码/修正
   并记录来源，而非只编译 relay。该原目录有未提交分析改动，逐文件核对，
   不直接 merge 整棵工作树，不修改原件。
3. **三内存档位的新派生 corpus。** Sabre full WS、No-image private WS、
   SplitSnap private WS、Full Dedup view 全部输出 streams8-v1；canonical chunk
   store、recipe、PFN/index 身份不变。新对象/endpoint/cache/results 使用新 namespace，
   不在旧正式 corpus 中放两代文件后依赖“哪个存在就读哪个”。
4. **shared base/rootfs/image sources 审计。** 当前主恢复的 shared source 是
   原有 coalesced raw content/index，不能误说它也是 private .zstd.frames。
   不改变 pinning、共享策略或把 shared 内容复制进每份 private WS。实际使用旧
   frames-v1 的大对象都要迁移，本来非分帧的 shared 对象不强制引入新压缩模式。
   离线 footprint 中按源压缩的 shared 项则必须采用新布局，并继续标为离线
   表示估算，不冒充 runtime RSS。
5. **新 cold aliases。** 原规则为 102 个逻辑点、85 个 distinct endpoint/revision
   （Pages/Sabre 共享基础 corpus）×60 aliases。别名不能复制混入旧 layout；
   可保留等价复用关系，不强求新实现物理 endpoint 数必须固定不变。
6. **正式 restore 全链路。** startup 配置→snapshot/recipe/WS metadata→
   Range readers→八 decoder→mmap staging→shared/private 页映射→UFFD→invocation。
   每步带 layout/config provenance；新主实验默认长流，不走旧 frame fallback。
7. **cache 生命周期。** 远端 miss、本地 hit、成功发布、计费/淘汰、失败释放接通。
   clean remote 不得以前一次缓存伪装 remote；all-local 同表示且 0 GET。
8. **完整 17×6 和 AES all-local。** 准备、环境控制、60-call 窗口、有效性检查及
   归档切到新 binary/corpus/config。短功能测试只是前置检查，不是交付终点。
9. **统计和图。** 重算 storage/cache/payload，解析新恢复日志，输出新 Figure9–13
   与 WS quality CSV；带宽结果另行获准后处理。核对未变的输入组成/WS 集合。旧 latency、旧压缩尺寸与新
   样本不得无标注混成一组；图表仍保留原件。

### 必须修改的代码入口

以下路径除标明工作树者外均相对 `snapshare-eval/`。

| 层 | 已确认的入口 | 所需改动 |
| --- | --- | --- |
| 编解码/运行 | 新分支 snapshotting/manager.go、snapshotting/zstdstream/frames.go、cmd/relay、cmd/snapshot_converter | 八长流格式、持续 decoder、新默认/格式选择、release/cache |
| MinIO/缓存 | 新分支 storage/minio_storage.go、workingSetCacheFileNames 等 | Range context、新后缀计数/计费、完整成功后发布 |
| 六系统转换 | prepare_figure9_16_systems.sh、prepare_figure9_16_current_base_tiered_systems.sh | 去除新配置中写死的 1-MiB 参数，三档 17 项完整转码 |
| Full Dedup | .dist/vhive-full-dedup-oracle/cmd/materialize_full_dedup_oracle_ws、prepare_figure9_16_full_dedup_oracle.sh | 替换旧 manifest/Decode/Encode 依赖，保持 canonical 验证，生成新 view |
| aliases/部署 | prepare_figure9_16_cold_aliases_direct.sh、restart_snapshare_xl170_worker.sh、远端 restart-worker helper | binary/layout 参数贯穿，别名格式一致 |
| 正式编排 | run_20260903_currentbase17_completion_pipeline.sh、run_figure9_16_direct_windows.sh、run_figure9_16_direct_window.sh | 派生新版入口，接受新目录/版本，完整 17×6，不重用旧 COMPLETE |
| footprint | analysis/figure9_10_currentbase17.go | 已转正 representative17 的实际入口；去除旧后缀及 Encode(...,1MiB,3) 假设 |
| payload/延迟 | collect_figure13_paper_payload.py、plot_figure13_paper_payload.py、Figure11/12 parser | 新对象尺寸/布局字段、计时事件兼容，保留 recipe 排除与归一化口径 |
| 敏感性/绘图 | Figure14 runner/parser、聚合 notebook/source scripts | 不复用旧布局 10G 点或旧 Full Dedup repeat；新数据/图另存 |

建议在新分支建立可版本管理的实验入口，如 `experiments/currentbase17-streams8/`，
放必要 Bash/Python/Go 工具或冻结依赖清单。结果和历史脚本留在原处；不覆盖历史
已运行脚本，也不把整个 snapshare-eval 或无关 IAA 改动盲目复制进系统仓库。

### 图表和 trace 的影响范围

| 产物 | 迁移后的处理 |
| --- | --- |
| Figure4 / background classification | raw 内容、页序和 source classification 未改时应不变；核对后可复用，不用重采快照 |
| Figure9/10 representative17 | 必须重算；private/full WS 用新实际压缩尺寸，shared/global union 用同一长流编码规则离线重编码，不能按旧压缩率缩放 |
| Figure11/12 | 用新 restore 结果生成；Figure11 包括 AES all-local，Figure12 完整 17 项 |
| Figure13 | 读取新 payload 大小，继续排除 recipe；Chunks/Pages 仍按其 unique native objects 计算，不用原始 WS 大小代替 |
| Figure14 | 另行纳入范围后测新实现的 1/2/5/10 Gbit/s 受影响行；10G 仅在同版本且满足无重叠/计时边界时可复用主实验，1 RPS 本身不是隔离保证。旧 25G 仍不恢复，不混新旧布局 |
| Figure15 WS quality | WS/PFNs 不变只保证覆盖集合不变，miss/latency 仍需新日志核对，不能假定所有指标不变 |
| Figure7/8 trace | 若把整套论文改称新主系统结果，受 codec/压缩 cache 容量影响的 trace 行也需重测；旧图可作为明确标注的历史版本保留，不能自动继承新实现结论 |

Figure9/10 保持 17 个代表性 workload、无 trace 频率/节点数倍乘；分母与去重
identity 规则不变。shared 源顺序/首次出现顺序不变，只换编码布局。实际 pinned raw
shared 和其离线 compressed footprint 必须继续区别说明。

trace 迁移要同步 loader/runtime 配置和逻辑身份隔离。r12 Full Dedup 本身以
scope-frame 为真实共享/计费/淘汰单位，不能把不同 scope 直接合成一个八流 WS：
这会改变额外取页和缓存粒度。若继续将它迁入新系统，应按同一 scope 的 canonical
对象规划长流并重建 PFN 引用，限制每次恢复最多八个并行 decoder，重新测读放大、
共享和淘汰；**不能保证每个跨 scope restore 总共只有八个 GET**。
这一依赖需单独验收，不应为了“八”而取消 Full Dedup 的共享语义；无需因此缩减
本轮完整 17-workload 单函数迁移范围。现在只记录流程，不启动 trace sweep。

## 5. 性能评估

### 对完整 currentbase17 的实际结构影响

本轮只读核对三档 `tier-*/oracle/validation.source.json` 的 17 个 revisions：
SplitSnap private WS 当前共 **330 帧**，每份 6–109 帧；所有对象均超过八页。
固定每对象八长流后是 **136 条流**，独立编码单元减少 58.8%。这是布局推导，
不是新实测性能或整次 restore 总 GET 数的减少率（后者还有 metadata/lazy pages）。

| Workload | 当前 private WS 帧数 | 目标流数 |
| --- | ---: | ---: |
| Image-Go | 10 | 8 |
| Image-Py | 109 | 8 |
| VideoProc | 33 | 8 |
| VideoAn | 51 | 8 |
| AES-Go / AES-Py / AES-NJS | 6 / 12 / 18 | 各 8 |
| Auth-Go / Auth-Py / Auth-NJS | 6 / 9 / 12 | 各 8 |
| Fib-Go / Fib-Py / Fib-NJS | 7 / 9 / 15 | 各 8 |
| Currency / Email / Product Catalog / Shipping | 9 / 10 / 7 / 7 | 各 8 |

12 个 workload 的数量下降，5 个由 6–7 增到 8。该表只统计 SplitSnap private
大对象；Sabre 完整 WS、No-image 和 Full Dedup 要在新 corpus inventory 中各自
列出原/新单元数及压缩大小，不能拿 private 数字代替六系统。
大对象保持一个 payload object，且流数固定不随 raw 大小增长，是实现验收项。

### 已有微基准只能提供性能线索

可行，但不能预设一定更快。已有同输入、同程序、同计时边界的 local A/B
如下（旧 private WS，每配置十次均值）：

| Workload | 1-MiB frames / W8，ms | 八条长流 / D8，ms | 长流耗时变化 |
| --- | ---: | ---: | ---: |
| AES-Go | 9.293 | 11.795 | +26.9% |
| Image-Go | 13.520 | 19.067 | +41.0% |
| Image-Py | 39.851 | 39.694 | −0.4% |
| VideoProc | 38.348 | 37.595 | −2.0% |
| VideoAn | 37.469 | 35.299 | −5.8% |

来源：[旧 r2 matched local 结果](../../../小测试/zstd-splitsnap-8worker/results/20260909_c6620_frames_vs_stream8_r2/RESULT.md)。
旧布局分别有 13/26/57/62/58 帧，新布局均为八流；
`小测试/zstd-splitsnap-8worker/main.go::encodeStreams` 可复用思路，但小测试
仍复用旧 manifest/Decode，并将 payload 存入 bytes.Buffer，不应原样搬到正式编码器。

最新 currentbase17 private WS 已有八长流 local 测量，五项为
5.400/7.972/58.794/19.741/28.161 ms，见
[最新输入及结果](../../../小测试/current-private-ws-codecs/results/20260910_currentbase17_private_zstd8_iaa11_r1/RESULT.md)。
这不是该新分支的网络测量，也没有同轮 currentbase17 小帧对照，不能跨输入算收益。

可能收益：大 WS 减少 GET 数、decoder 创建与任务调度；更长上下文可能改善压缩率。
风险：raw 等分不保证解码耗时相等，慢流可能拖尾；窗口/RSS 可能增加；小 WS
强制拆八份可能反而更慢。当前不足八帧的 WS，新布局甚至可能增加 GET 数量。

若链路带宽为 B、各流压缩大小为 C_i，总网络工作仍受 sum(C_i)/B 限制；
八 GET 不会把链路带宽变成八倍。完成时间还受最慢流、解码、RTT、内存/缓存影响。
Figure11 蓝段还有 metadata、recipe、WS index 等，不能将 private WS 改进率
直接当成完整 fetch+decompression 或 E2E 改进率。

结论：基础 API 已齐备，**codec 原型风险较低，manager/cache/corpus 集成是主要工作**。
值得做真实 MinIO A/B，目前没有证据保证所有 workload 或 E2E 都改善。

## 6. 实施待办（以主系统迁移完成为终点）

下列 P1–P4 未勾选项是设计时的最终验收清单，**不表示当前没有候选实现**。
已有实现/局部证据与真正剩余项以 STATUS、实施冻结稿 §14 为准，不从零重复工作。

### P0：评估与隔离（设计阶段记录）

- [x] 核对正式 Range/Decode 路径及既有八流小测试。
- [x] 从正式单函数基线创建独立分支 `likun/splitsnap-sort`。
- [x] 记录格式、输出/缓存边界、已有性能反例和实施范围。
- [x] 最初设计阶段未改原代码/服务/图；已有后续候选进展另见 STATUS，本轮只改文档。
- [x] 修正为完整 17×6 + AES all-local 范围，核对各系统和实际 17 份 private 帧数。
- [x] 补齐 converter→corpus→aliases→restore/cache→runner→统计/绘图的依赖。

### P1：codec 与必要集成

- [ ] 新增 `snapshotting/zstdstreams/`（建议名）：流式编码、八项索引、八路 Decode；
  固定 klauspost/compress v1.17.11，不同时升级库。
- [ ] 编码直接写目标文件、页对齐八等分，不产生逐 1-MiB EncodeAll/Reset 任务。
- [ ] relay/converter 增加明确布局选项，如 `-zstdWSLayout streams8-v1`；
  新版主实验默认该布局，不自动 fallback，流数不借用旧 zstdFrameSize 参数。
  旧 decoder 只保留给离线迁移/显式旧版对照，不在新主路径按文件存在性偷偷选择。
- [ ] manager 接入 persist/read、cache inventory/计费/淘汰、完整发布和 release；
  shared sources、private index、policy 不变。
- [ ] MinIO 新后缀进入 payload/metadata counter；父 context 贯穿真实请求。
- [ ] 更新 corpus 转换/别名文件白名单，独立新前缀输出；保留旧 `.zstd.frames`。
  native Chunks/Pages 的压缩与对象粒度暂不修改。
- [ ] 新正式 compressed/coalesced restore 从 LoadSnapshot 传 context；移除
  private→full、整对象下载及上层吞错等隐式回退，保留 source/learning 合法无 WS。
- [ ] 异步 compressed cache 使用临时文件完整发布及 lifecycle token，防止
  半文件命中/淘汰后旧任务重新发布；细节见实施冻结稿。
- [ ] 少量 focused tests：roundtrip、页/流边界、空/小 WS、截断/损坏、range
  延迟/失败取消、并发写目标区间、缓存成功发布/失败不命中。
- [ ] 延迟 reader 证明输入未收完即可产生输出；不以并行下载后 DecodeAll 通过验收。
- [ ] Full Dedup materializer 与 representative17 统计工具纳入新分支可构建依赖，
  同步接口/后缀/编码规则；不把 Full Dedup 留到“以后也许推广”。

### P2：完整 corpus、配置和缓存接通

- [ ] 独立 launcher/cache root/完整 relay endpoint 贯通；不能直接调用会停止
  通用服务的旧 restart/all-local helper。处理 native 行也传旧 frame-size 参数的
  启动问题。具体只读审计与接口约定见实施冻结稿 §7。
- [ ] 新 corpus 选择性复制、逐 revision 转码记录及部分失败处理；不镜像旧 aliases，
  不复用旧 COMPLETE。Full Dedup 保持正式 `security=partial` transfer-view 行。
- [ ] 准备容量包含基础对象、新 WS、60-call aliases、临时文件与对象开销；
  不将排除 WS 的 scan bytes 当作完整需求，不假设 server-side copy 零空间。
- [ ] 确认 currentbase17 三档原始输入可用；需要时离线恢复旧压缩 raw，冻结全部
  17 份 snapshot/WS/index 和完整请求矩阵，不只冻结五个 private 文件。
- [ ] 对全部 Sabre/No-image/SplitSnap/Full Dedup coalesced WS 重新编码；Chunks/
  Pages native store 保持等价。生成 per-system/per-workload 的格式/大小清单。
- [ ] 在新配置里定义布局和 stream 数；检查 converter、relay、restart helper
  与 runner 参数真正一致。已核对旧正式 single-function runner 显式传 W8，
  源码默认 W10 不是该轮实测设置；新对照同为八路，不改变其它基线参数。
- [ ] 完整新 aliases 与 102 个逻辑配置映射；新命名不携带旧 frames 的隐式依赖。
- [ ] 少量小/大 WS 的真实 MinIO→decoder→UFFD 验证后，进入完整矩阵，避免再做
  多轮重复五 workload 微基准。核对 parent context、输出释放及 cache 发布。
- [ ] AES all-local 使用同布局缓存，0 GET；cache-hit、miss、淘汰的计费与新尺寸一致。

### P3：完整 17×6 正式主实验

- [ ] 准备完成后在同一 worker/backend 上对齐 2.1GHz、SMT/Turbo off、10 Gbit/s、
  `-j16`、三档 VM 内存、请求与 backend cache 策略；上传准备可不限速。
  正式 runner 检查 worker `ifb0` 入站 TBF、backend 无重复 TBF；复核新 MinIO
  端口的过滤器覆盖，不能只记录一个“10G”数值而忽略限速位置和参数。
- [ ] 运行 17 workload × Chunks/Pages/Sabre/SplitSnap−/SplitSnap/Full Dedup；
  原采样定义每点 30 warmup + 30 measured，共 6,120 calls；AES all-local 再 60，
  总 6,180。均为计划规模，本轮没有启动。
- [ ] cadence 可配置并记录。复现旧主实验须保留/说明其 1-RPS 定义；隔离 fetch
  对照和带宽实验必须验证无重叠。若原 cadence 已过载，应两边统一修正并说明，
  不一边用新隔离请求、一边直接拿旧过载点声称纯 codec 收益。
- [ ] 保留全部样本/错误，核对每点应用成功、layout=streams8-v1、payload/index
  覆盖、PFN 数、实际 GET、内存/CPU、释放和 cache 状态；不能只用进程退出码。
- [ ] 给出整体 fetch+decode、metadata/index、UFFD、restore、E2E。每流记录首读/
  首输出/结束等轻量信息，不把八路时间相加当 wall，或用减法拆重叠的 GET/decode。
- [ ] 旧正式结果保留作历史比较。需要归因纯布局差异时，在同环境额外做少量
  旧 W8/新 D8 matched A/B；不自动再完整重复一套旧 6,180 calls。

### P4：完整统计、敏感性和版本交付

- [ ] representative17 Figure9/10 重算：实际 payload 尺寸、shared/global union
  的新编码结果、相同 identity/分母；保留 metadata 排除和 oracle 说明。
- [ ] Figure11/12 新完整日志；Figure13 新内容 payload，继续全部排除 recipe。
  collector 的 logical_restore_bytes 可能含 recipe，图仍应取 compressed_payload_bytes。
- [ ] 【单列发布依赖，需用户明确纳入 goal】Figure14 受影响的 17-workload 行用
  新版测量覆盖现有 1/2/5/10G 范围，保持隔离调用；新主实验 10G 点只有经核实
  无重叠、输入和计时边界也一致才可复用，不能只因为带宽相同直接替代。
  本轮不启动，也不默认加入下一次 17×6 主矩阵的执行终点。
- [ ] 复核 Figure4 与 WS 集合不变，Figure15 的运行时指标由新日志更新。
- [ ] 同步新版 notebook/图说明与汇总表。新结果/图另存，旧正式图不静默覆盖。
- [ ] 明确 trace Figure7/8 是否仍引用旧版；若发布为同一新版系统结果，先完成
  trace/runtime/corpus 适配与必要重测，含 r12 scope 缓存的独立语义检查。

## 7. 完成标准

完整 17×6 加 AES all-local 的主系统路径、corpus/config/aliases、缓存和统计都
使用一致新布局；所有适用的大 WS 对象不再生成随大小增长的 1-MiB 小帧，
每对象只保留最多八条独立长流及索引。真实 MinIO、八 decoder、mmap/UFFD 生命周期
正确，交付完整矩阵的结果、分析和复现入口（含退化项）。

仅创建分支、跑通 codec/五个 workload、或只改 private WS 而漏掉 Sabre/Full Dedup，
都不算完成。性能是否提高由整套实测决定，不以必须获胜作为样本筛选条件。
trace 的状态单独报告，不能以 17-workload 完成冒充十节点 trace 也已迁移。

上述勾选保留设计时状态；已有候选与证据见 SPLITSNAP_SORT_STATUS.md，当前只做
文档审计。下一次 goal 按冻结稿 §14.4 接续，不按未勾选状态从零重复已完成的工作。
