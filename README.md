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

## 可选：启用系统原生 BBR + FQ

该项是安装完成后的可选系统网络优化，不属于 VPSKit 当前受管功能。VPSKit 继续保持默认不修改内核、BBR、通用 `sysctl` 和系统网络拥塞控制配置。

适用边界：

- Debian 12/13、Ubuntu 24.04 等现代发行版通常已经包含原生 BBR；
- 这里只使用当前系统内核已有的原生 BBR，不安装第三方内核、BBRv3、自定义内核，也不修改 GRUB 或删除现有内核；
- 不会修改 VPSKit 的 REALITY / Hysteria2 节点凭据，不会改变节点地址、端口、UUID、REALITY key 或订阅 URL；
- 如果节点配置本身没有变化，正常情况下不需要重新安装 VPSKit，也不需要重新导入客户端配置或更新订阅；
- BBR 主要作用于 TCP，因此主要影响 REALITY / TCP 等 TCP 流量；Hysteria2 基于 UDP/QUIC，不直接使用 TCP BBR；
- 不应把它宣传为“必然降低延迟”或“必然提高速度”，实际收益取决于线路、拥塞和带宽环境。

推荐理解为：

```text
VPSKit 安装
    ↓
节点正常工作
    ↓
可选手动开启 BBR + FQ
```

先做只读检查：

```bash
uname -r
sysctl net.ipv4.tcp_congestion_control
sysctl net.core.default_qdisc
modinfo tcp_bbr 2>/dev/null || true
modinfo sch_fq 2>/dev/null || true
ip route show default
tc qdisc show
```

启用前先记录原始值，并保存实际输出；不要假定原值一定是某个固定算法或固定 qdisc：

```bash
sysctl net.ipv4.tcp_congestion_control
sysctl net.core.default_qdisc
```

如果此时已经显示正在使用：

```text
bbr
fq
```

则通常无需重复配置。不要只依赖 `lsmod`、`sysctl net.core.default_qdisc` 或单个接口现状作为唯一判断依据。

其中 `net.core.default_qdisc=fq` 表示系统为后续创建的网络设备队列设置默认 qdisc。它不保证所有当前网卡的 root qdisc 都直接显示为 `fq`，也不保证多队列设备不会显示 `mq`，或虚拟接口不会显示其他 qdisc / `noqueue`。`tc qdisc show` 应作为补充观察，而不是“当前所有网卡都已使用 FQ”的单一证明。

如需启用，先准备模块并确认 `bbr` 已进入可用拥塞控制列表：

```bash
sudo modprobe tcp_bbr
sudo modprobe sch_fq
sysctl net.ipv4.tcp_available_congestion_control
```

只有在加载模块后仍然看不到 `bbr` 时，才应停止并确认当前内核或模块环境是否满足原生 BBR 条件；不要继续写入 BBR 的 `sysctl` 配置。

确认 `bbr` 可用后，再写入独立配置文件：

```bash
sudo tee /etc/sysctl.d/99-vpskit-bbr.conf >/dev/null <<'EOF'
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF

sudo sysctl --system
```

然后验证：

```bash
sysctl net.ipv4.tcp_congestion_control
sysctl net.core.default_qdisc
sysctl net.ipv4.tcp_available_congestion_control
```

预期主要结果：

```text
net.ipv4.tcp_congestion_control = bbr
net.core.default_qdisc = fq
```

回退方式：

```bash
sudo rm -f /etc/sysctl.d/99-vpskit-bbr.conf
sudo sysctl --system
sysctl net.ipv4.tcp_congestion_control
sysctl net.core.default_qdisc
```

如果当前值已经恢复到启用 BBR + FQ 前记录的原始值，则回退完成，无需继续操作。

如果重新加载系统中剩余的持久化 `sysctl` 配置后，当前值仍未恢复，而你又明确需要立即恢复到启用前记录的运行时状态，可以执行：

```bash
sudo sysctl -w net.ipv4.tcp_congestion_control=<原始拥塞控制算法>
sudo sysctl -w net.core.default_qdisc=<原始qdisc>
```

文档中的 `<原始拥塞控制算法>` 和 `<原始qdisc>` 必须替换为你启用前记录的真实值，不要写死成 `cubic`、`fq_codel` 或其他猜测值。

最后的 `sysctl -w` 只修改当前运行中的内核参数，不创建持久化配置。因此，当前运行时可以恢复为之前记录的值；系统重启后的最终值仍由内核 / 发行版默认值、`/etc/sysctl.conf`、`/etc/sysctl.d/*.conf`、`/usr/lib/sysctl.d/*.conf` 以及系统中其他仍存在的持久化 `sysctl` 配置共同决定。

如果你没有记录原始值，不要猜测；删除 `99-vpskit-bbr.conf` 并执行 `sudo sysctl --system` 后，如需彻底重新建立系统启动状态，可以重启 VPS，但不要擅自指定 `cubic`、`fq_codel` 或其他假定恢复值。

安全与操作边界：

- 不执行来源不明的远程脚本；
- 不自动修改额外 TCP buffer、`tcp_rmem`、`tcp_wmem` 等激进参数；
- 对生产 VPS 操作前，建议保留服务商控制台或救援入口；
- 原生 BBR + FQ 本身不需要更换内核，正常情况下也不需要重启系统。

更完整的安装后操作说明见[安装与首次使用](docs/INSTALL.md)。

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
