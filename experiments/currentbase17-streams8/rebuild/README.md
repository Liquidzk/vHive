# 从远程源码重新构建：辅助入口

主指南：[SPLITSNAP_REMOTE_REPRODUCTION.md](../../../docs/SPLITSNAP_REMOTE_REPRODUCTION.md)。

- `build.sh`：构建 relay、converter、oracle materializer、八长流准备/核算工具和
  pinned direct-invoker；只写调用者指定的新输出目录，不安装服务。
- `build-guest.sh`：从仓库已有的 Git LFS SquashFS 派生可用的 guest，添加实验
  registry 的 HTTP resolver 和操作者自己的 SSH 公钥。不能传私钥。
- `legacy-workspace/snapshare-eval/`：把原来不属于任何 Git 仓库的准备源码纳入
  远程。保留其原始语义/目录结构用于迁移，不是已经可直接执行的新节点 installer。
  **不要直接运行其中的 bootstrap、restart 或 stage 脚本**：它们仍引用历史节点、
  本地镜像归档和固定工作目录；部分还会操作专属服务、网络或 devmapper。

历史源码中 `minio/minio123` 等是隔离实验网的固定测试凭据，不适用于公开服务。
这里不包含 nodes.env、私钥、旧日志、结果、snapshot、数据库 dump、OCI 或二进制。

准备源码索引：

| 文件 | 作用 |
| --- | --- |
| `configure_c6620_cpu_baseline.sh` | c6620 的 SMT/Turbo/2.1 GHz 设置与验证 |
| `bootstrap_zstd_streaming_c6620_two_node.sh` | 原 backend/worker 服务配置参考；不是新节点完整 installer |
| `bootstrap_c6620_worker_assets.sh` | worker runtime 配置/安装参考；历史跨节点复制源已不可用 |
| `restart_snapshare_xl170_worker.sh` | guest、runtime、relay 参数参考；文件名不是当前硬件型号 |
| `rebuild_function_images.sh` | 从 vSwarm 源码重建四个 restore-safe workload normal/eStargz 镜像 |
| `stage_figure9_16_direct_assets.sh` | 原资产部署位置参考；新跑不能继续依赖旧 snapshots.zip |
| `prepare_figure9_16_current_base_tiers.sh` / `prepare_figure9_16_inputs.sh` | 按三档内存重新采集 base、17 snapshots 和五次 restore-union WS |
| `run_figure9_16_call.sh` | source、profile、source-local 的直接请求协议 |
| `prepare_figure9_16_current_base_tiered_systems.sh` / `prepare_figure9_16_systems.sh` | 同一 raw corpus 转换各 baseline |
| `prepare_figure9_16_full_dedup_oracle.sh` | canonical union 与 runtime transfer view |
| `configs/figure9_16/` | 17 workload、请求、六系统和调用配置模板 |

上述历史准备源码未修改。新的 run 必须先按主指南迁移环境引用，并生成自己的
输入清单与校验记录。不能复制一个旧 COMPLETE/release JSON 来冒充本轮运行成功。
