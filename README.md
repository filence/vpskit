# VPSKit

VPSKit 是一个面向个人 VPS 的低资源、可回滚代理节点部署与生命周期管理工具。当前生产架构由 Xray 承载 VLESS REALITY TCP 入站，由 sing-box 承载 Hysteria2 UDP 入站；两者可以共用同一个数字端口，例如 TCP/443 与 UDP/443。

公开仓库：[filence/vpskit](https://github.com/filence/vpskit)

> 当前正式版本：[`v0.2.0`](https://github.com/filence/vpskit/releases/tag/v0.2.0)。发布资产包含 checksums、Ed25519 签名清单、SPDX SBOM、显式 Linux 权限归档与 GitHub attestation；安装器只接受该固定 Release，不执行 `main` 分支脚本。

`v0.2.0` 将已完成实机验收的 v0.1.1 运维基础与 v0.2 系列功能统一收口：Cloudflare Workers/KV 自动订阅、ACL4SSR 分流与 anti-AD、受控规则例外、Fail2ban SSH 白名单、Hysteria2 实验增强、UDP buffer 与端口跳跃，以及通用实例 Adapter 生命周期。完整证据见 [`v0.2.0 实机验收报告`](docs/V0.2.0_ACCEPTANCE.md)，版本变更见 [`v0.2.0 Release Notes`](docs/releases/v0.2.0.md)。

## v0.2.0 核心能力

- **固定、安全的部署与升级**：签名安装包、事务备份/回滚、`doctor --fix`、系统检查、清理与脱敏诊断包；
- **自动订阅**：Cloudflare Workers/KV 发布 Mihomo 完整配置、v2rayN 节点订阅和 manifest，支持 ETag、Token 轮换/吊销、远端回读及静态配置兜底；
- **分流与去广告**：受管 ACL4SSR、anti-AD 规则缓存、原子刷新失败保护，以及可审计的精确白名单和自定义 `DIRECT / PROXY / REJECT` 规则；
- **Hysteria2 增强**：只读能力/性能检查、保守 UDP buffer 档、默认关闭的 Salamander，以及独立 nftables/systemd 组件实现的可回滚端口跳跃；
- **边界清晰**：不默认修改 SSH、内核、BBR、通用防火墙或云安全组；不提供 Web 面板、多租户或流量计费。

## 已验证范围

- Debian 13 amd64；
- Xray `v26.3.27`：VLESS + REALITY + Vision；
- sing-box `v1.13.14`：Hysteria2 + TLS；
- ZeroSSL/Let's Encrypt ACME + Cloudflare DNS-01；
- Clash Verge、Hiddify、Mihomo、sing-box 与 Xray 客户端；
- 安装、诊断、实例变更、证书续期、备份恢复、自更新、核心更新、卸载重装和重启持久化。

其他平台的证据等级见 [兼容性说明](docs/COMPATIBILITY.md)。

## v0.2.0 一键安装

在Debian 13 amd64 VPS的Bash中执行：

```bash
curl --fail --location --proto '=https' --tlsv1.2 \
  --output install.sh \
  'https://github.com/filence/vpskit/releases/download/v0.2.0/install.sh'
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
sudo vpskit instance list
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
sudo vpskit hysteria2 inspect
sudo vpskit hysteria2 performance inspect
sudo vpskit hysteria2 udp-buffer status
sudo vpskit hysteria2 port-hop status
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

Cloudflare 账户级管理 Token 不得复制到 VPS；VPS 只接收受 `node_id` 约束的发布 Secret 与订阅读取 Token。此组命令要求 v0.2.0 或更高二进制。

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
