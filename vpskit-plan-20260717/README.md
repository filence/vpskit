# VPSKit 自研一键搭建脚本方案包

版本：v1.1 源码审核修订版

日期：2026-07-17

源码与上游核验截止：2026-07-17

目标环境：Debian 12/13、Ubuntu 24.04；amd64/arm64；systemd

低配基准：1 vCPU、1 GB RAM、约 10 GB 磁盘

公开仓库：[filence/vpskit](https://github.com/filence/vpskit)

## 方案结论

VPSKit 当前生产方案采用 **双核心、双独立入站**：

- `Xray v26.3.27` 承载 `VLESS + TCP/RAW + REALITY + Vision` TCP 入站；
- `sing-box v1.13.14` 承载 `Hysteria2 + TLS` UDP 入站；
- 两者可以共用同一个数字端口，例如 TCP 443 与 UDP 443；
- Mihomo 不作为 VPS 常驻服务，而作为客户端 YAML/订阅导出目标；
- Xray 已作为 REALITY 生产 Adapter 纳入签名发布包；原因是固定版本实机 A/B 测试中，Mihomo/Xray/sing-box 客户端均能连接 Xray REALITY 服务端，而 Mihomo 与 Xray 客户端不能通过 sing-box REALITY 服务端认证；
- 不安装 Docker、Web 面板、数据库、Caddy/Nginx 等常驻组件，优先降低资源和维护成本。

源码审核后新增的硬约束：

- sing-box 生产基线固定到经过审核的稳定 Release；不能使用其默认 `testing` 分支；
- Mihomo 只能按 Release tag/对应发布分支核验；不能从当前默认 `main` 获取代理内核源码；
- Bootstrap 使用小型 Bash 脚本，主命令发布为无常驻的 Go 单文件程序；不在 VPS 上依赖 Python 或动态插件运行时；
- Cloudflare DNS-01 由固定版本的外部 `lego` Provider 执行，不绑定 sing-box 内联 ACME 字段；
- v0.1 不加载第三方动态插件，所有 Adapter/Provider/Renderer 都是编译期内部接口；
- 运行单元使用独立 `vpskit-xray.service` 与 `vpskit-sing-box.service`、同一非 root 用户和最小能力集；
- 版本锁必须由 VPSKit 自己签名的发布清单背书，不能只信任与资产同源下载的摘要文件。

### ACME 证书颁发机构

首个正式Bootstrap默认选择已完成实机路径验证的ZeroSSL，也允许在向导中选择Let's Encrypt。低级CLI在未指定环境时仍以Let's Encrypt为库级默认；生产安装应以向导中显示的CA为准：

```bash
export VPSKIT_ACME_SERVER=zerossl
export VPSKIT_ACME_EMAIL=your-email@example.com
```

VPSKit 会根据邮箱调用 ZeroSSL 的 EAB 接口；如果已经在 ZeroSSL 控制台生成 EAB，也可以直接提供：

```bash
export VPSKIT_ACME_EAB_KID='...'
export VPSKIT_ACME_EAB_HMAC='...'
```

EAB 不写入发布包、不进入备份，安装后仅保存在 VPS 的 `/var/lib/vpskit/secrets/acme.env`（权限 `0600`），续期沿用同一 CA 配置。不要把真实 EAB 或 API 凭据写入仓库或聊天记录。

## 2026-07-20 lab31 实机状态

- Debian 13 amd64 已完成 Xray REALITY + sing-box Hysteria2 双核心 clean install，TCP/UDP 可共用 443；
- `doctor`、配置摘要、密钥一致性、双协议回环握手、实例启停、备份/恢复、卸载/重装、自更新、核心版本检查、orphan scan 与重启持久化均通过；
- `www.amazon.com:443` 在 VPS 侧扫描为 TLS 1.3、h2、X25519MLKEM768，证书有效，REALITY 候选验证通过；
- 固定版本 sing-box `v1.13.14`、Mihomo `v1.19.29` 与 Xray `v26.3.27` 客户端已完成自动化握手；
- Xray 与 sing-box 空闲 RSS 在安装后及收口审计中观测约 83.5–107.8 MB，低于 120 MB 目标；
- 重装后最终凭据已在 Clash Verge 与 Hiddify 中完成 REALITY、Hysteria2 四项人工验收；剩余工作属于正式多系统、长时间运行和运维观察矩阵，不影响本台 Debian 13 实验 VPS 的 lab31 验收结论。
- 验收后已清除 VPS 重复恢复副本、旧 lab30 安装包和本机五批过期客户端配置；当前只保留一份受管回滚备份与四份最终客户端配置。
- lab32 签名开发包已将远程安装迁移至 schema 5，并完成无效目标零写入、REALITY 目标切换后恢复亚马逊、修订号1→3、安全 ZIP、双协议回环和Windows固定版本解析；最终修订3配置已在 Clash Verge 与 Hiddify 中通过 REALITY、Hysteria2 四项重新导入验收。
- lab33 已完成固定版本Bootstrap、显式Linux归档权限、原位自更新和中文管理菜单实机回归；随后在同一VPS创建并下载可校验恢复快照，执行受管卸载与最终Bootstrap从零重装。摘要/签名、schema 5初始修订、安全ZIP、doctor、证书、orphan scan、双协议回环及重启持久化通过，新配置已在Clash Verge与Hiddify完成REALITY、Hysteria2四项公网GUI验收。验收后已按用户确认删除远程11项恢复/安装临时材料与本机恢复目录，节点健康，accepted客户端配置保留。受保护GitHub工作流只会创建草稿Release，生产环境与密钥仍须在建仓后配置。

## 当前交付物

- [VPSKit-完整方案-v1.1-20260717.md](VPSKit-完整方案-v1.1-20260717.md)：单文件同步版；
- [VPSKit-完整方案-v1.1-20260717.zip](../VPSKit-完整方案-v1.1-20260717.zip)：文档包，包含拆分文档、完整版与源码快照清单。

原 v1.0 单文件与 ZIP 已保留在项目根目录 `_archive/VPSKit-v1.0-20260717/`，不再作为活动版本。

## 文档目录

1. [01-完整方案与产品边界.md](01-完整方案与产品边界.md)
   项目目标、首版功能、使用流程、安全边界、目录结构和命令设计。

2. [02-架构与扩展接口设计.md](02-架构与扩展接口设计.md)
   状态模型、事务引擎、适配器接口、渲染器接口，以及后续协议/面板/Docker/WARP 等扩展方式。

3. [03-低配资源预算与运行策略.md](03-低配资源预算与运行策略.md)
   面向 1C1G/10G 磁盘 VPS 的内存、CPU、磁盘、日志、备份和升级预算。

4. [04-开发路线与验收标准.md](04-开发路线与验收标准.md)
   分阶段开发任务、测试矩阵、失败回滚、发布门槛和最终验收清单。

5. [05-参考项目与社区调研.md](05-参考项目与社区调研.md)
   官方项目、现有一键脚本、NodeSeek、BWGSS、JHXIE、V2RaySSR 等来源的评审记录。

6. [06-源码审核与修订意见.md](06-源码审核与修订意见.md)
   本次本地源码审核结论、必须修订项、已采纳决策和仍待实机验证的风险。

本地源码快照位于项目根目录 `source-references/`。源码仓库不进入方案 ZIP，也不部署到 VPS；ZIP 只包含文档和源码快照清单。

## 推荐阅读顺序

首次阅读：`README → 06 → 01 → 03 → 02 → 04 → 05`。

准备交给 Codex 开发时，重点提供：

- `01-完整方案与产品边界.md`
- `02-架构与扩展接口设计.md`
- `04-开发路线与验收标准.md`
- `06-源码审核与修订意见.md`

## 当前边界

本方案同时是设计规范和当前实现基线。已完成的阶段必须以签名包、自动化回归和实机证据为准；尚未通过对应验收矩阵的系统与功能仍只能标记为“目标支持”。

本次源码审核确认的参考基线包括 sing-box `v1.13.14`、Mihomo `v1.19.28`、Xray-core `v26.3.27` 和 lego `v5.2.2`。这些版本只表示 2026-07-17 的审核快照，不表示未来可永久不变；实际安装版本由每个 VPSKit Release 自带的签名锁文件决定。
