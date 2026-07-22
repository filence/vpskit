# Changelog

本项目在正式版本出现前使用实验版本号；实验版本不构成稳定兼容承诺。

## Unreleased

- `v0.2.4-lab.2`：新增 `vpskit security fail2ban whitelist list|add|remove --cidr <IP-or-CIDR> --yes`。白名单仅作用于 VPSKit 所有的 Debian `sshd` jail 覆盖文件；IP/CIDR 会规范化、去重并限制为最多 32 条，手工改动或所有权不匹配时拒绝覆盖。
- 白名单变更在 `fail2ban-client -d` 校验后重启服务，并最多等待 15 秒确认 `sshd` jail 控制 socket 已就绪；失败自动写回原覆盖文件并重启 Fail2ban。Debian 13 实机已完成保留地址添加、jail 回读、删除、空状态以及 Xray/sing-box active 回归。
- `v0.2.3-lab.1`：状态 schema 升至 9，新增 `vpskit rules whitelist <list|add|remove>` 和 `vpskit rules custom <list|check|add|remove>`。白名单仅允许精确 `DOMAIN,DIRECT`；自定义规则支持 `domain`、`domain-suffix`、`ip-cidr` 与 `direct`、`proxy`、`reject`。
- 用户规则以受管状态保存，在 Mihomo 中稳定排在远程 ACL4SSR/anti-AD Provider 前；域名/CIDR 会规范化，重复或同目标冲突规则会被拒绝，`custom check` 可报告范围重叠提示。每次规则例外变更都创建事务备份、递增客户端修订并自动发布；规则缓存 revision 不被虚增，以保持受管缓存引用正确。
- Debian 13 amd64 已完成 schema 8→9 原位迁移、白名单/自定义规则添加和删除、订阅 r0012 全目标回读及 Xray/sing-box active 回归；测试使用 `.invalid` 保留域名，最终状态不保留测试规则。
- `v0.2.2-lab.2`：新增 `vpskit rules refresh --yes`。它下载 ACL4SSR、MetaCubeX Google 与 anti-AD 规则，执行每源 1 MiB 上限与 SHA-256 记录，按 ruleset revision 缓存，并把 Mihomo 主配置、v2rayN、manifest 与最多 20 个规则工件作为完整修订发布到 Workers/KV。
- Workers 订阅后端支持经读取 Token 保护的 `/s/<token>/rules/<name>`；发布、回读和回滚以候选修订中实际的 target 集合为准，并保留旧三工件修订回滚兼容。
- 发布请求上限提高至 2 MiB；规则工件发布的默认 HTTP 等待时间提高至 90 秒。实机发现旧 20 秒等待不足时会返回 `DEGRADED`，现已通过 r0008 重新发布和 Windows 11 Clash Verge Rev 更新/切换/实际连接验收。
- 新增现有 Worker 的只读检查和原位更新脚本；更新保留 KV 绑定与 Worker Secret，不轮换读取 Token 或节点发布 Secret。
- v0.2.0 单 VPS 自动订阅高优先级基础已完成实机验收：Cloudflare Workers/KV Publisher、节点级 HMAC 发布、不可变修订、旧修订拒绝覆盖、回读收敛、Mihomo/v2rayN/manifest 固定入口、ETag/HEAD/304、读取 Token 轮换/吊销、发布回滚和静态导出降级均已打通。
- 新增 `vpskit subscription plan/configure/publish/status/rotate-read-token/revoke-read-token/rollback/remove`；订阅故障只标记 `DEGRADED`，不会回滚已成功的节点配置或停止 Xray/sing-box。
- 新增固定 Wrangler `4.112.0` 的 Cloudflare 部署脚本；脚本将 Cloudflare 管理 Token 与下发 VPS 的节点发布凭据分离，并拒绝隐式覆盖已有 Worker 或凭据文件。预发与正式 Worker/KV/Custom Domain 都已通过实机健康检查；Win11 Clash Verge Rev 与 v2rayN 订阅 URL 更新均已通过人工验收。
- v0.1.1 Renderer 与运维基础已完成：状态 schema 6、节点元数据、结构化 Mihomo、统一 Artifact Set、Renderer capability、static Publisher、迁移预演、清理计划、系统检查和脱敏故障包均已实现。
- Debian 13 amd64 已完成 v0.1.0→v0.1.1 原位升级、回滚、再次更新、schema 迁移、配置修订、双协议回环和服务重启回归；修复了已运行服务在配置变更后未重新载入的问题。
- Win11 Clash Verge Rev `v2.5.2` / Mihomo `v1.19.29` 完成完整 YAML GUI 导入和 REALITY/Hysteria2 延迟测试；v2rayN `v7.23.4` 完成节点兼容核验，并对 `v7.23.1` 官方解析源码进行了对照。
- v0.1.1 的完整证据边界记录于 `docs/V0.1.1_ACCEPTANCE.md`；自动订阅仍由 v0.2.0 实施。

- `v0.1.0`公开版本已完成普通用户从零安装、客户端ZIP下载、Clash Verge/Hiddify导入及REALITY/Hysteria2实际连接验收，第一阶段正式收尾。
- 方案拆分文档和单文件同步版继续保留在源码仓库；重复的方案ZIP改为本地按需生成，不再作为Git源码对象跟踪。
- 状态升级至schema 5，新增客户端配置修订号；旧schema 4安装在内存迁移时从修订1开始。
- 新增 `vpskit export --format bundle`：生成权限为 `0600`、带SHA-256清单和离线说明的客户端ZIP，默认交给发起 `sudo` 的SSH用户，拒绝覆盖同名文件。
- `vpskit export` 与 `status` 返回当前配置修订号。
- `vpskit instance modify reality --reality-server-name <domain>` 支持事务化更换REALITY目标；变更前执行TLS和端到端验证，成功后重新生成客户端配置并明确返回需要更新的客户端字段与回滚备份ID。
- 首发继续不开放持久在线订阅端口；静态YAML、JSON、分享链接和二维码在修订变化后需要重新导出。
- lab32 已在 Debian 13 amd64 实验 VPS 完成 schema 5 迁移、无效目标零写入、REALITY 目标切换并恢复、原凭据不变、配置修订1→3、安全 ZIP清单和双协议回环；下载后的最终 ZIP 已通过固定版本 sing-box/Mihomo 解析，并在 Clash Verge 与 Hiddify 中完成 REALITY、Hysteria2 四项 GUI 重新导入验收。
- 参数化13个历史lab脚本中的实验域名、Cloudflare Zone和VPS IPv4；本地SSH包装器只从被忽略的环境文档注入，公开工作树个人环境值扫描归零。
- 新增固定版本 `install.sh` 模板、PowerShell/Go双生成入口和 `--verify-only`；归档先校验SHA-256，再校验内置公钥签名清单，初始安装拒绝覆盖已有状态。
- 新增 `vpskit menu` 中文管理入口，提供状态、诊断、客户端导出、二维码、REALITY扫描/更换、备份、证书和中断恢复；变更类操作要求精确大写确认。
- 发布归档改由Go工具写入显式Linux权限，修复Windows打包导致可执行位丢失的问题；新增受保护GitHub工作流，先做无密钥质量门和构建证明，再只创建草稿Release。
- lab33 已在同一Debian 13 amd64实验VPS完成Bootstrap摘要/签名验证、原位自更新、doctor、中文菜单只读操作、REALITY真实扫描和取消变更零写入回归。
- lab33 最终回归先创建并下载可校验恢复快照，再对同一VPS执行受管卸载和固定Bootstrap从零重装；schema 5初始修订、证书、orphan scan、安全ZIP、双协议回环和重启持久化通过，新配置随后在Clash Verge与Hiddify中完成REALITY、Hysteria2四项公网GUI验收。
- lab33 人工验收后按用户确认删除远程11项恢复/安装临时材料与本机恢复目录，复核节点健康并保留accepted客户端配置。

## v0.1.0-lab.31 - 2026-07-20

- 生产架构改为 Xray `v26.3.27` 承载 REALITY、sing-box `v1.13.14` 承载 Hysteria2，状态升级至 schema 4。
- 签名发布包扩展为 13 个资产，`versions.lock` 同时锁定 Xray、sing-box 与 lego，并补齐 Xray MPL-2.0 NOTICE/许可证。
- balanced 安装支持 `existing-files` 证书输入；外部证书只读校验后复制到受管目录，不改外部文件权限。
- ZeroSSL EAB 与 Cloudflare Token 仅通过进程环境传给 lego，不进入 argv；发行包、审计输出与客户端导出均不包含 ACME 凭据。
- REALITY 目标扫描器支持候选列表、TLS/ALPN/密钥交换/证书/延迟检查，并可执行 sing-box REALITY 端到端验证。
- 完成双核心状态、服务、备份恢复、实例启停、自更新、核心已是当前版本、卸载重装、重启持久化和 orphan scan 回归。
- Debian 13 amd64 实机 clean install 通过；Xray 与 sing-box 空闲 RSS 在安装后及收口审计中合计约 83.5–107.8 MB，低于 120 MB 目标。
- 最终导出已由 sing-box `v1.13.14`、Mihomo `v1.19.29`、Xray `v26.3.27` 执行固定版本握手测试。
- 最终重装凭据已由用户在 Clash Verge 与 Hiddify 中分别验证 REALITY、Hysteria2，四项人工验收全部通过。
- 验收后删除 VPS 上的重复恢复副本与旧 lab30 安装包，清理本机五批过期客户端配置；保留一份 lab31 受管回滚备份和四份最终配置。

## Earlier labs

- 新增 Reality-only 初始安装 Profile，不申请证书、不创建 ACME 密钥或续期定时器。
- 新增 Reality/Hysteria2 单实例启用、禁用、端口修改和删除事务，失败自动回滚。
- 新增终端二维码导出，默认不回显原始分享链接。
- 新增 Linux amd64/arm64 构建、固定版本静态分析、Secret scanning 与 SPDX SBOM CI 基建。
