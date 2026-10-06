# Gate12 八长流：源码恢复与结果复现入口

2026-09-11：完整103点和五图已完成，本指南的源码恢复/完整aggregate已经实际执行。
原始命令、退出0与分析版本见
[gate12-full-aggregation-r1.json](provenance/20260910-r1/gate12-full-aggregation-r1.json)，
数值、文件检查及最终审计见 [正式图包](results/20260910-r1-gate12-figures-r1/README.md)。
下面的重绘只读原始结果，不是要求重跑实验；旧服务、输入和结果均应保留。

## 1. 版本组成，不能只用基线commit

| 层次 | 固定入口 | 含义 |
| --- | --- | --- |
| 基线源码 | `3f18475f5f5665848d1e4f7745d9c92e75b1a7ec` | 不包含八长流dirty候选 |
| Gate12运行源码/工具 | `provenance/20260910-r1/gate12-release.json`，完整`source-complete-overlay-r1`与`toolset-r1` | 包含converter/runtime/cache、准备/runner/validator与procfs竞态修正；运行binary身份由release与build-info记录 |
| 后处理 | 已在gate12完整source-r1中 | 含此前style/METHODS/FINDINGS及平台检查、gate12版本支持；不要再叠加旧gate11分析包覆盖它 |
| 运行后补充文档 | 本指南、METHODS及STATUS | 描述实际选定版本和接续；与冻结运行源码分开，不反写旧archive/release/frozen |

这些材料恢复的是候选源码，不含所有guest镜像、原始corpus或外部worker服务。
本指南在运行源码包之后补充，不反写已冻结release或改变正在测量的binary。

## 2. 在独立临时目录恢复源码（不碰工作树/服务）

以下命令从原仓库读取已存在commit，仅向新建临时目录解包；路径按当前工作区写出。
后面的代码块在同一shell中使用这些任务专用变量。

```bash
repro_repo=/home/liquid/invitro-related/.dist/vhive-splitsnap-sort
repro_bundle="$repro_repo/experiments/currentbase17-streams8/provenance/20260910-r1"
repro_root=$(mktemp -d /tmp/splitsnap-streams8-source.XXXXXX)
git -C "$repro_repo" archive 3f18475f5f5665848d1e4f7745d9c92e75b1a7ec | tar -xf - -C "$repro_root"
tar -xzf "$repro_bundle/gate12-source-complete-overlay-r1.tar.gz" -C "$repro_root"
tar -xzf "$repro_bundle/gate12-delivery-docs-r1.tar.gz" -C "$repro_root"
```

gate12 source-r1已包含`config/20260910-r1/plan.json`、workloads/requests和本地分析所需Python
源码；Go基线中的相对替换`./examples/protobuf/helloworld`也在基线archive中。
不需要把另一个worktree的最新文件混进来。
最后一层仅补交付文档，不含运行/分析代码，也不改变frozen或统计公式。
它更新归档时METHODS的旧gate11来源说明，并补当前结果/审计/复现入口。

历史gate11曾按base→r3→analysis-findings-r1在独立临时目录恢复，对aggregate/findings/metrics/accounting/
runner/launcher及plan逐文件cmp一致，聚合入口`--help`退出0。历史证据为
[`reproduction-source-check-r1.json`](provenance/20260910-r1/reproduction-source-check-r1.json)。
这只验证当时的源码层叠与Python入口，不是全套结果重绘或位级binary重建证明。
本轮gate12已核对worker binary、三节点各43个工具文件及本地105个非Markdown
源码成员一致，见 `gate12-deployment-source-check-r1.json`。下面的源码恢复检查
另记于 `gate12-reproduction-source-check-r1.json`，不把旧恢复记录重标成新版结果。

若需要功能重建relay，在上面的独立目录、具备Go/C编译依赖的环境中执行：

```bash
cd "$repro_root"
mkdir -p bin
GOTOOLCHAIN=go1.26.7 CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GOAMD64=v1 go build -buildvcs=false -o bin/relay-streams8-rebuilt ./cmd/relay
```

Go1.26.7/CGO1/linux/amd64/v1来自已保存`gate12-build-info.txt`；依赖使用原go.mod/
go.sum（包括klauspost/compress v1.17.11），不执行`go get -u`。这不是声称不同
绝对路径、工具链/链接环境下能得到相同binary hash。精确复用正式窗口时使用
release记录的原binary；重新构建/改变版本需要独立结果版本，不重标旧样本。

## 3. 从已收齐的103点离线重绘与统计

这里只读已收结果，不调用远端实验。需要保留整个`results/20260910-r1-gate12/`，
不只是`POINT_COMPLETE.json`：每点invocations、relay日志、fetch/final/platform、
shutdown/resources等均由聚合器读取或供最终审计；新版还逐点复核三节点before/after
平台快照（CPU/SMT/Turbo/10G-IFB/无重复backend TBF及先后顺序），缺失或不符则
在输出前失败。`frozen.json`内包含静态报告与
输入/aliases/release。单独下载图片无法复现统计。

```bash
cd "$repro_root/experiments/currentbase17-streams8"
repro_output_parent=$(mktemp -d /tmp/splitsnap-streams8-replot.XXXXXX)
python3 aggregate.py --results "$repro_repo/experiments/currentbase17-streams8/results/20260910-r1-gate12" --output "$repro_output_parent/figures"
```

输出目录必须尚不存在；不覆盖旧图。缺任何正式点会在创建输出前失败，不能改plan
或用`--static-only`绕过。输出包括Figure9–13 PNG/PDF和CSV、逐调用/median/WS
quality/资源CSV、README、METHODS和FINDINGS；FINDINGS保留退化，不能替代人工
的逐项完成审计。固定分析环境见`aggregation-environment.json`：Python3.12.3、
Matplotlib3.10.8、NumPy2.4.3、Agg；新环境变化另记，不改旧记录。

实际首次完整聚合使用恢复目录 `/tmp/splitsnap-streams8-gate12-source.UF9e2U`，
输出到 `results/20260910-r1-gate12-figures-r1`；该目录现已存在，不能再次覆盖。
已检查8份CSV、五个PNG/单页PDF、全部103点与3,090measured及618份平台记录。
上述命令用独立临时输出，避免第二次重绘覆盖首次交付。PDF创建时间可能不同，
数值复现以CSV及公式/来源一致为准，不要求PNG/PDF文件哈希逐位一致。
首次聚合时尚未叠加文档增量，之后仅修正输出METHODS的来源段落；该文件现在
与本地E/METHODS完全相同。所有数值仍来自该run的frozen，没有修改它。

## 4. 重新运行不是“解包后直接再发103点”

- 先核实同一`streams8_formal_gate12_matrix_r1`的live进程和终态；未知/SSH超时
  不能视作停止，已发窗口只补收，不重放。完整命令不使用缩小范围的`--only`。
- 复用实际inventory/aliases之前核对run/layout/端点和剩余空间；当前三档/六系统
  共102行、5100aliases，原WS/PFN/分类保持。准备I/O不能与正式测量重叠。
- 外部invoker来源见`external-direct-invoker.json`及其源码包；worker依赖见
  `external-worker-runtime.json`与配置包。两个worker构建标记`modified=true`，
  必须保留匹配的binary，不能仅以嵌入commit声称可以重建。guest kernel/rootfs/
  镜像与原corpus是单独依赖，记录路径不等于完整离线副本已在本包里。
- 新节点要重新核实地址/设备/服务与隔离资源，不盲用本轮硬编码端点或停止原服务。
  CPU/SMT/Turbo与worker10G-IFB及无backend重复TBF的检查由正式runner逐点记录。
- gate12的缺WS、真实LoadSnapshot后取消、整个unit7VMM退出已保存同版证据。
  不用gate11通过记录代替；原小/大WS和all-local功能证据按版本与不变codec边界
  保留；新的AES正式all-local已在本矩阵完成60calls，store禁用、0远端读取。
  当前完整范围仍是
  17×6 remote + matched SplitSnap AES all-local，每点60次、30warmup+30measured。
  BW、C3/r12 trace不因本指南存在就变成已迁移/已重跑。

## 5. 旧结果与诊断工具

gate11前35点是其旧版本数据，第36点仍INVALID，不重标/混入gate12。恢复旧源码
应使用gate11-release指定的base→source-r3→analysis-platform-r1，而不是gate12
运行源码。两代artifact分开索引，不靠文件名或图中标签隐去版本变化。

`retire_gate11_networks.py`是本轮一次性的精确资源收尾工具，不在gate12测量
路径中；其诊断receipt与源码另存，不要求复现者运行它。不要复制该工具去清理
新节点，更不能对namespace/服务使用通配符。原服务、数据和图片始终保留。
