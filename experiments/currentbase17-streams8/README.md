# currentbase17：八长 Range / 八 decoder 主实验

## 当前交付（2026-09-11）

gate12完整103/103点、6,180calls/3,090measured已完成，controller退出0。
从冻结source-r1真实恢复源码后重验、绘图，新包为
[20260910-r1-gate12-figures-r1](results/20260910-r1-gate12-figures-r1/README.md)。
包含Figure9–13 PNG/PDF、8份CSV、METHODS/FINDINGS、数值核验；原图未覆盖。
[最终逐项审计](../../docs/SPLITSNAP_SORT_FINAL_AUDIT.md)与
[复现指南](REPRODUCE.md)是当前入口。运行版为gate12完整source/toolset-r1，
不要把下方历史gate11分析包覆盖它，也不要照历史命令再发窗口。
AES all-local同表示/0远端读取，额外UFFD长尾和较慢workload完整保留。

## 以下为实现语义及历史接续记录

分支 `likun/splitsnap-sort`，正式基线 `3f18475f` 加本工作树候选；本地已有 gate12 版本入口/release。
完整目标：17 workload × 6 systems 的 102 个 remote 点，加 matched SplitSnap
AES-Go all-local。每点 60 calls，slots 0–29 warmup、30–59 measured。
状态以 [STATUS](../../docs/SPLITSNAP_SORT_STATUS.md) 为准，不由目录存在推断完成。

**当前用户goal continuation恢复完整实施。** gate12三节点工具文件、worker
二进制和本地105个非Markdown源码归档成员已核对一致，见
`provenance/20260910-r1/gate12-deployment-source-check-r1.json`。
旧失败点17个网络已精确退役；原服务/网络/输入/结果保留，不重标gate11前35点。
同版缺WS、LoadSnapshot后取消及整unit7VMM正常退出均通过。完整矩阵已启动于
`streams8_formal_gate12_matrix_r1` / `results/20260910-r1-gate12/controller-matrix-r1`，
无`--only`；实时进度见STATUS，完整103点仍待收齐。
后续验收仍完整103点，不按下方历史完成数或gate9示例重放窗口。
最新本地分析增量为analysis-platform-r1，含之前排版/METHODS/FINDINGS和逐点平台
离线检查；正式发布前必须核对全部before/after快照，详见REPRODUCE及其manifest。
完整[验收索引](../../docs/SPLITSNAP_SORT_ACCEPTANCE.md) 不以局部点代替103点。
最新细节见 [实施稿 §0](../../docs/SPLITSNAP_SORT_IMPLEMENTATION.md#0-本次确认与下一次-goal-待办)。
源码恢复、完整结果离线重绘及外部依赖边界见 [REPRODUCE.md](REPRODUCE.md)；
不要只用基线commit，或照历史gate9单点命令重启当前矩阵。

本地已有 gate11 negative 七个 VMM 的 shutdown、完整源码 overlay-r3/文件清单/
release、含 late-cancel/accounting 的 toolset-r3 和 Python40/OK 历史日志。
runner 将静态报告/release 嵌入 frozen.json，正式聚合使用该快照；这些不是本轮
新增实施。旧 gate9 INVALID 保留，不把局部验收当作完整结果。
历史矩阵为 `streams8_formal_gate11_matrix_r1`，结果根
`results/20260910-r1-gate11/`；历史记录为Python3318838/pane3318832、无`--only`，
本轮未查询它们，后续获准先核对同一任务，不按旧PID/完成数直接重启。

补充外部loader来源见 `provenance/20260910-r1/external-direct-invoker.json`；当前
二进制与原currentbase17 receipt同SHA，已归档其提交源码与同SHA本地build-info。
它不在relay源码overlay内，也不是本轮更换的八流实现；该补充不改写frozen.json。
外部worker四个运行binary、三份配置和本地分析环境分别见同目录
`external-worker-runtime.json`、`external-worker-configs-r1.tar.gz` 与
`aggregation-environment.json`。两个外部组件构建标记modified=true，复现依赖
精确保留的binary而非仅嵌入commit；本轮没有重建/替换它们或修改运行配置。

## 固定语义

- 一个 coalesced payload 内八条页对齐独立长流；每流一个长 Range reader、一个
  concurrency=1 decoder，直接写 host raw mmap 的对应区间，再进入既有 UFFD。
  PFN/raw/classification 不变；SHA/CRC 保留，不是去校验的 codec-only 微基准。
- Sabre full WS、No-image/SplitSnap private WS、Full Dedup partial-format oracle
  transfer view 都用同布局。Chunks 128 KiB / Pages 4 KiB 原生路径不变。
- Full Dedup footprint 使用 canonical union；runtime 使用预生成 transfer view，
  offline assembly 不计入延迟。其 payload 可与 SplitSnap 完全相同。
- 独立 run `20260910-r1`，worker er020、backend er069、loader er032；worker
  8090/8091、sstr/172.30+172.31，新 MinIO 956x/957x/958x。复用 shared runtime
  服务，但不停止旧 relay/FC/containerd/MinIO/DB/registry 或清空其缓存。
- 主矩阵为 absolute 1 RPS，HTTP/teardown 可以重叠并留记录；不是隔离 BW 测试。
  三台 c6620、2.1 GHz/SMT off，worker IFB 10 Gbit/s；probe 在正式点前后留证据。

## 输入与已有产物

`config/20260910-r1/plan.json` 冻结系统、请求和三档映射。
`provenance/20260910-r1/actual-layout.json` 有 102 行；`aliases-complete.json`
有 5,100 aliases。已有 copy/transcode/oracle/footprint/payload 报告，不无故重建。
`results/20260910-r1-static-r1/` 是既有新 Figure9/10/13 静态 CSV/PNG/PDF，
不是完整性能包。旧 gate7 AES INVALID；gate8 AES remote 已通过但与 gate9 分开。

历史 gate11 发布索引：`provenance/20260910-r1/gate11-release.json`，源码包
`gate11-source-complete-overlay-r3.tar.gz` / `gate11-source-files-r3.txt`，工具包
`gate11-toolset-r3.tar.gz`。源码包对基线 3f18475f 的完整改动覆盖，不依赖 gate10
增量包；binary 仍为 `bin/relay-streams8-gate11`，工具目录仍 `toolset-gate11/`。
后续文档更新不覆盖已归档的 r3 源码/运行配置。

本地后处理新增`gate11-analysis-style-r1.json/.tar.gz`：仅aggregate.py的Figure10
标题换行和刻度排版，源码增量从仓库根叠加在r3之上；不部署至测量节点，不重写
runtime release/frozen。`results/20260910-r1-static-r2/`为新静态预览，数值CSV
与r1完全相同，5项聚合/来源tests通过；最终新图须同时注明这份分析增量。

此前分析增量为`gate11-analysis-methods-r1.json/.tar.gz`，直接叠加source-overlay-r3，
已包含上面的style改动，并增加自动随结果导出的[METHODS.md](METHODS.md)。不要再在
其上覆盖旧style增量。static-r3已验证文档复制成功且两份CSV与static-r2相同；
5项聚合/来源tests通过。它不更换runtime/toolset或改写frozen；最终输出须标注此
分析版本，解释实际对象容量与模型、Figure11重分类/残差、SHA/并行计时和资源口径。

最新分析增量是`gate11-analysis-findings-r1.json/.tar.gz`，包含此前style/METHODS，
新增findings.py及定向测试。正式aggregate在全103点验收后自动输出并链接FINDINGS.md，
列同轮六系统raw-mean比较、每workload退化、容量/payload减少率及归因限制；
static-only不生成该性能说明。8tests通过，tar compare rc0；这是本地后处理，
不改变正在运行的binary/toolset/frozen，不生成残缺正式包。直接叠加source-overlay-r3，
不要再把旧style/methods包覆盖其上。

历史 gate9 二进制：`bin/relay-streams8-gate9`（仓库根）；工具部署到每台节点
`/users/Liquidz/streams8/20260910-r1/toolset-gate9/`。
完整候选 overlay 为 `provenance/20260910-r1/gate9-source-complete-overlay-r2.tar.gz`，
文件列表为 `gate9-source-files-r2.txt`；在基线 3f18475f 上覆盖这些文件即可恢复
归档时的源码/配置，二进制构建来源见 gate9-build-info。已有运行依赖和 raw
corpus 不包含在源码包里，分别由 config/provenance 及既有环境提供。
不要只凭基线 commit 重建 dirty binary。

## 后续 goal 获准后的正式接续（本轮不执行）

按实施稿 §0.3 先核对同一历史任务/收集状态，以及实际候选、部署、release 版本。
gate11 的整 unit 负向退出已有记录，但不能代替 gate12 同版缺 WS/晚期取消/整池
退出验收。采用 gate12 时使用其 release 指定的完整源码/toolset-r1，不能照旧
“必须使用 gate11/r3”的文字覆盖它；当前部署及验收本轮未核实。
正式结果与部署/静态来源同版，不改名混入 gate9/11 点。完整源码包不包括外部
corpus/运行依赖；最终复现仍须对账。本轮不改变旧 release、归档和 frozen.json。
Go 文件级 race / Python40 历史通过不代表全仓通过，ctriface 整包旧 API 编译失败
仍如实披露。下方 gate9 命令仅保留历史用法，不能按其中 `--only` 缩减完整范围。

先查 STATUS 所指本地 controller 和远端同一 job/unit 的真实存续、rc 与收集状态。
未知/观测超时不重放窗口；只收集已有任务。启动必须放在新的 controller attempt
目录，本地 tmux 运行下列入口；此示例不自动启动实验：

```bash
bash run-controller.sh "$PWD/results/20260910-r1-gate9/controller-ATTEMPT" \
  --output "$PWD/results/20260910-r1-gate9" --version gate9 \
  --only splitsnap-zstd3:aes-go-45000-45450:remote
```

上述 `--only` 只是选择点的语法示例。gate9 AES remote/local 已有 60-call 局部
验收记录，历史上也已启动不带 `--only` 的 matrix controller。获准后先核实该
controller/job 的真实状态，不能照示例重复启动 AES 或再开并行 controller。
安全接续时已通过点重新审计后跳过，不重复发请求；终点仍为 102 remote + 1 local。

每点收集原始调用、relay、platform-before/after、最终 idle 与 counters、VM 关联
验证，全部通过才写 POINT_COMPLETE；随后只退出该点自己的已空闲服务。
失败保存原始日志及 unsettled 诊断，不把未收尾 counters 当正式 final。
大量调用小文件在窗口结束后以一次压缩 tar 流收回；单文件传输也启用压缩，
不改变窗口内的 MinIO 请求、压缩或计时。退出后另保存 relay-resources.json：
systemd 服务全生命周期 CPU time/内存峰值，含启动与收集等待，不是单次 codec
开销，也不包含其它服务 cgroup 的 guest/MinIO 资源。正式聚合要求该记录完整。

## 聚合

新 runner 已将完整 footprint/payload 报告与 inventory 对账并嵌入 frozen.json，
正式 aggregate 只读冻结内容；static-only 预览才读固定 provenance。完整 103 点
通过后，从匹配版本结果聚合，新输出目录必须不存在。旧缺 accounting 的结果根
不会静默回退到可变文件，也不能自动当成新版完整包。
下列 gate9 命令仅说明历史参数形式，不作为新正式结果的推荐版本：

```bash
python3 aggregate.py --results results/20260910-r1-gate9 \
  --output results/20260910-r1-gate9-figures
```

- Figure9/10：跨 tier 的首次出现 union，非三个压缩报告之和。
- Figure11：measured median components；嵌套 WS timer 不重复加到蓝段。
- Figure12：relay E2E median，归一化到 Pages。
- Figure13：compressed content payload，排除 recipe/index/manifest；归一化到 Chunks。
- Figure12/13 Mean 均先平均 17 个原始标量再归一化，不平均比值。
- 整窗 60-call counter 不除以 30 冒充测量均值。BW/C3/r12 不混入或重标。
- Figure15 runtime 质量另存 CSV：额外 UFFD events = handled − 1；保留所有
  尾部样本，全 WS/private 数量须与 inventory 匹配。whole_ws_pct 不含 recipe，
  legacy_private_plus_recipe_pct 仅保留明确命名的旧图口径；它不是 Figure13。
- relay-lifetime-resources.csv 只汇总上述生命周期诊断，不当作解压性能图。

正式模式先验收全部点再创建输出。完成条件仍包括数值/图/来源与实现文档要求的
逐项验收；脚本退出 0 或一幅图存在不单独证明整个 goal 完成。
