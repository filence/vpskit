# VPSKit

VPSKit 是一个面向个人 VPS 的低资源、可回滚代理节点部署与生命周期管理工具。当前生产架构由 Xray 承载 VLESS REALITY TCP 入站，由 sing-box 承载 Hysteria2 UDP 入站；两者可以共用同一个数字端口，例如 TCP/443 与 UDP/443。

公开仓库：[filence/vpskit](https://github.com/filence/vpskit)

> 当前正式版本：[`v0.1.0`](https://github.com/filence/vpskit/releases/tag/v0.1.0)。发布资产已经通过checksums、Ed25519签名清单、SPDX SBOM、Linux权限和GitHub attestation复核；公开版本已经完成普通用户从零部署、客户端ZIP下载及REALITY/Hysteria2实际连接验收。

第一阶段已于2026-07-20完成收尾，结论和证据边界见[`v0.1.0 第一阶段实机验收报告`](docs/PHASE1_ACCEPTANCE.md)。

`v0.1.1` Renderer 与运维基础已在实现分支完成，包含 schema 6 节点元数据、结构化 Mihomo、统一 Artifact/Publisher、迁移预演、清理、系统检查和脱敏故障包，并已通过 Debian 13 amd64 升级/回滚及 Win11 Clash Verge/v2rayN 验收。当前公开稳定安装入口仍保持 `v0.1.0`，直到后续统一发布流程完成；证据见[`v0.1.1 实施验收报告`](docs/V0.1.1_ACCEPTANCE.md)。

`v0.2.0-lab.1` 的单 VPS 自动订阅 MVP 已完成 Cloudflare 预发布和正式后端的实机部署。它包含 Workers/KV 发布端、Mihomo 完整配置订阅、v2rayN 节点订阅、Token 轮换/吊销、远端回读和回滚；Windows 11 上的 Clash Verge Rev 与 v2rayN 已完成订阅导入和更新实测。详见 [`v0.2.0 自动订阅实机验收报告`](docs/V0.2.0_ACCEPTANCE.md)；公开 GitHub Release 仍须另行完成发布门禁。后端部署说明见 [Cloudflare Worker README](deploy/cloudflare/README.md)。

`v0.2.2-lab.2` 已在同一 Debian 13 amd64 VPS 完成 ACL4SSR + anti-AD 方案 A 的受管规则发布。VPSKit 会下载、限额检查并哈希 20 个规则源，按 ruleset revision 缓存后与主订阅一起发布到 Workers/KV；Clash Verge Rev 已完成 r0008 更新、切换和实际连接验收。Worker 更新保留既有 KV、读取 Token 和节点发布 Secret；发布器已将多工件请求等待时间提高到 90 秒，以覆盖规则工件写入与回读。

`v0.2.3-lab.1` 已完成 schema 8→9 的原位迁移和规则例外生命周期。它支持精确域名白名单及域名、域名后缀、IP CIDR 的自定义 `DIRECT / PROXY / REJECT` 规则；规则始终位于 anti-AD 前面，变更会创建备份、递增客户端修订并自动发布。VPS 已完成添加、发布、删除和 r0012 回读闭环；真实误杀域名的 Clash Verge 命中验证仍按需进行。

`v0.2.4-lab.2` 已完成 Fail2ban SSH 白名单。它仅更新 VPSKit 自己的 `sshd` 覆盖文件，接受规范化的单一 IP 或 CIDR，最多保存 32 条；每次变更先校验 Fail2ban 配置、重启服务、等待 SSH jail 控制 socket 就绪并回读，失败则自动恢复原文件和服务。Debian 13 已完成“添加保留测试地址 → jail active → 删除 → 空白名单”的实机闭环；代理服务和订阅配置未改动。

`v0.2.5-lab.1` 将受管规则刷新改为临时目录下载、完整校验后原子激活。上游任一规则源不可达、返回错误或超出限额时，当前缓存、订阅修订和客户端配置保持不变，临时目录自动清除。Debian 13 已使用仅对该命令生效的无效代理模拟上游失败，确认状态、订阅、当前规则清单和 Xray/sing-box 服务均未改变。

`v0.2.6-lab.1` 新增只读 `vpskit hysteria2 inspect`。它报告当前 sing-box、UDP 监听和系统 UDP 缓冲，并按锁定的服务端与 Windows Mihomo 能力矩阵标记功能：Salamander 为实验性、Gecko 与 BBR profile 因 sing-box 需至少 1.14 而阻止、端口跳跃因需独立 NAT/云安全组/回滚组件而阻止。Debian 13 实机验证该命令不修改状态或订阅，Xray/sing-box 保持 active。

lab32 已在同一实验 VPS 完成 schema 5 迁移、无效 REALITY 目标零写入、目标切换并恢复、修订号递增、安全 ZIP、双协议回环及本地固定版本解析；修订3配置随后在 Clash Verge 与 Hiddify 中完成 REALITY、Hysteria2 四项 GUI 重新导入验收。

lab33 继续完成固定版本Bootstrap、Linux归档权限、原位自更新与中文菜单实机回归；随后在同一VPS创建本机可校验恢复快照，执行受管卸载与最终Bootstrap从零重装。签名/摘要校验、schema 5初始修订、安全客户端ZIP、doctor、证书、orphan scan、双协议回环和重启持久化均通过；新修订配置已再次通过Clash Verge与Hiddify的REALITY、Hysteria2四项人工验收。验收后已删除远程恢复/安装临时材料和本机恢复副本，仅保留本机accepted客户端配置。

## 已验证范围

- Debian 13 amd64；
- Xray `v26.3.27`：VLESS + REALITY + Vision；
- sing-box `v1.13.14`：Hysteria2 + TLS；
- ZeroSSL/Let's Encrypt ACME + Cloudflare DNS-01；
- Clash Verge、Hiddify、Mihomo、sing-box 与 Xray 客户端；
- 安装、诊断、实例变更、证书续期、备份恢复、自更新、核心更新、卸载重装和重启持久化。

其他平台的证据等级见 [兼容性说明](docs/COMPATIBILITY.md)。

## v0.1.0 一键安装

在Debian 13 amd64 VPS的Bash中执行：

```bash
curl --fail --location --proto '=https' --tlsv1.2 \
  --output install.sh \
  'https://github.com/filence/vpskit/releases/download/v0.1.0/install.sh'
sudo bash install.sh
```

安装器采用固定版本下载、签名校验和中文引导流程：

```text
下载固定 Release
→ 校验发布清单和签名
→ 环境预检
→ 选择 balanced 或 reality-only
→ 验证 REALITY 目标
→ 显示变更计划并确认
→ 事务化安装
→ 自动诊断
→ 导出客户端配置
```

Bootstrap 不会默认修改 SSH、内核、BBR、系统防火墙或云安全组。不要执行来自 `main` 分支的脚本，也不推荐 `curl | bash`；上面的两步命令会先保存固定版本安装器，再由用户明确运行。完整前置条件、在线/离线安装和客户端交付见[安装与首次使用](docs/INSTALL.md)。

## 当前管理命令

```bash
sudo vpskit menu
sudo vpskit status
sudo vpskit doctor
sudo vpskit doctor --fix
sudo vpskit reality scan
sudo vpskit backup
sudo vpskit cert status
sudo vpskit cert renew
sudo vpskit update self --bundle-dir /绝对路径/签名发布包
sudo vpskit update core --bundle-dir /绝对路径/签名发布包
sudo vpskit node show
sudo vpskit migrate check
sudo vpskit cleanup plan
sudo vpskit system inspect
sudo vpskit system updates
sudo vpskit rules show
sudo vpskit rules whitelist list
sudo vpskit rules custom list
sudo vpskit rules custom check
sudo vpskit security fail2ban status
sudo vpskit security fail2ban plan
sudo vpskit support bundle
```

出现 anti-AD 误杀时，先从 Clash Verge 日志确认被拒绝的域名，再仅添加精确白名单：

```bash
sudo vpskit rules whitelist add --domain captcha.example.com --yes
sudo vpskit rules whitelist remove --domain captcha.example.com --yes
```

需要自行指定路由时，可添加受校验的域名、后缀或 IP 段规则；每次变更都会发布新的 Mihomo 订阅修订：

```bash
sudo vpskit rules custom add --type domain-suffix --value example.org --policy proxy --yes
sudo vpskit rules custom remove --type domain-suffix --value example.org --policy proxy --yes
```

如需避免自己的固定管理出口被 SSH 防护误封，可仅添加明确可信的单个 IP 或 CIDR；不要把宽泛公网网段加入白名单：

```bash
sudo vpskit security fail2ban whitelist list
sudo vpskit security fail2ban whitelist add --cidr 203.0.113.10 --yes
sudo vpskit security fail2ban whitelist remove --cidr 203.0.113.10 --yes
```

修改 REALITY 目标时，VPSKit会先执行TLS和端到端REALITY验证，再创建回滚备份、重新渲染服务端与客户端配置并递增配置修订号：

```bash
sudo vpskit instance modify reality \
  --reality-server-name www.amazon.com
```

也可以同时修改REALITY TCP端口：

```bash
sudo vpskit instance modify reality \
  --reality-server-name www.amazon.com \
  --port 8443
```

成功结果会返回 `client_update_required: true`、`config_revision`、`changed_client_fields` 和 `previous_backup_id`。客户端完成新配置验证前不要删除该回滚点。

## 客户端配置

生成仅当前SSH用户可读取的ZIP配置包：

```bash
sudo vpskit export --format bundle
```

默认输出到发起 `sudo` 的SSH用户主目录，权限为 `0600`，内容包括：

```text
vpskit-client-r0001-YYYYMMDD-HHMMSS.zip
├── mihomo.yaml
├── sing-box-reality.json
├── sing-box-hysteria2.json
├── share-links.txt
├── manifest.json
└── README.txt
```

Reality-only部署不会包含Hysteria2客户端文件。使用SCP或SFTP下载后，分别将 `mihomo.yaml` 导入Clash Verge，将分享链接或JSON导入Hiddify/sing-box。

终端二维码不会回显原始链接：

```bash
sudo vpskit export --format qr
```

静态文件、分享链接和二维码不会自动更新。已配置 v0.2.0 Workers 订阅的节点则可通过 Mihomo 或 v2rayN 订阅 URL 更新；未配置订阅时，修订号变化后仍必须重新导出并导入。完整说明见[客户端配置导出与更新](docs/CLIENT_CONFIGS.md)。

在 v0.2.0 中，受信任的本地管理端先部署 Cloudflare 后端，再在 VPS 安装节点级凭据并发布：

```bash
sudo vpskit subscription plan --credentials-file /root/vpskit-subscription.json
sudo vpskit subscription configure --credentials-file /root/vpskit-subscription.json --yes
sudo vpskit subscription publish
sudo vpskit subscription status
```

Cloudflare 账户级管理 Token 不得复制到 VPS；VPS 只接收受 `node_id` 约束的发布 Secret 与订阅读取 Token。此组命令要求 v0.2.0 或更高二进制，不应在公开 `v0.1.0` 二进制上执行。

## 安全边界

- Cloudflare Token、ACME EAB、REALITY私钥和客户端凭据不得写入仓库或命令行参数；
- 发布包必须通过内置公钥验证签名清单；
- 配置变更先创建受管备份，失败自动回滚；
- 卸载只处理ownership记录内的受管对象；
- 客户端ZIP包含凭据，下载并导入成功后应删除VPS上的临时副本。
- `scripts/lab` 只通过环境变量接收实验域名、Zone和IPv4，仓库不提供开发者个人环境默认值。

安全问题请参阅 [SECURITY.md](SECURITY.md)，发布门禁见 [docs/RELEASE.md](docs/RELEASE.md)，首次提交与公开Release的人工步骤见[GitHub发布清单](docs/GITHUB_PUBLISH_CHECKLIST.md)。

## 开发验证

```bash
go mod verify
find cmd internal -type f -name '*.go' -print0 | xargs -0 gofmt -l
go vet ./...
go test ./...
staticcheck ./...
```

GitHub Actions还会执行Linux amd64/arm64构建、固定版本客户端解析、ShellCheck、Gitleaks和SPDX SBOM生成。受保护发布工作流只创建草稿 Release，人工复核后才能公开。设计规范和实验记录位于 [vpskit-plan-20260717](vpskit-plan-20260717/README.md)；方案Markdown保留在源码仓库，重复的二进制文档ZIP仅按需在本地生成，不纳入Git跟踪。
