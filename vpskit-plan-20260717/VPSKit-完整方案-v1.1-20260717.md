# VPSKit 自研一键搭建脚本完整方案

版本：v1.1 源码审核修订版

日期：2026-07-17

低配基准：1 vCPU、1 GB RAM、约 10 GB 磁盘

本文件由 01–06 拆分文档按编号顺序生成。若内容不一致，以同版本拆分文档和 README 为准。

---

# VPSKit 完整方案与产品边界

版本：v1.1 源码审核修订版
日期：2026-07-17

## 1. 项目定位

VPSKit 是一个面向个人 VPS 的、低资源占用的代理节点部署与生命周期管理工具。

它的职责是：

1. 安装和管理经过版本锁定的代理核心；
2. 创建、修改、检查、导出和删除受管节点实例；
3. 对配置和二进制升级执行事务化检查、切换与回滚；
4. 输出 Mihomo、sing-box 和标准分享链接所需的客户端配置；
5. 只修改明确属于 VPSKit 的文件、服务和防火墙规则。

它不是：

- VPS 万能工具箱；
- Web 管理面板；
- Docker 管理器；
- 建站环境；
- 系统 DD、内核魔改或网络“玄学优化”集合；
- 多租户机场管理系统。

## 2. 首版技术结论

### 2.1 默认运行模式

首版默认只运行一个 `sing-box` systemd 服务。产品支持两个独立入站，但不要求初始化时必须一次创建两个：

| 入站                       | 传输       | 安全机制      | 默认用途        |
| ------------------------ | -------- | --------- | ----------- |
| VLESS + REALITY + Vision | TCP/RAW  | REALITY   | 稳定主节点       |
| Hysteria2 + TLS          | UDP/QUIC | 正常 TLS 证书 | 丢包或移动网络备用节点 |

这两个入站不能合并成一个协议连接，但可以处于同一份 sing-box 配置、同一个进程中。

首版提供两个 Profile：

- `reality`：最低前置条件，只创建 VLESS REALITY；
- `balanced`：在证书已就绪或 DNS-01 可完成时，同时创建 Reality 与 Hysteria2。

`balanced` 是推荐组合，不是无条件默认。没有域名、证书或可用 UDP 时，脚本必须允许先部署 `reality`，不能为了“完整双节点”强迫用户开启跳过证书验证。

### 2.2 端口策略

默认建议：

```text
TCP 443 → VLESS REALITY Vision
UDP 443 → Hysteria2 TLS
```

TCP 443 与 UDP 443 是不同的套接字，可以同时监听。安装前必须分别检查：

```bash
ss -ltnp   # TCP
ss -lunp   # UDP
```

冲突处理：

- TCP 443 已占用：Reality 改用其他 TCP 端口，或由用户明确选择迁移现有服务；
- UDP 443 已占用：Hysteria2 改用其他 UDP 端口；
- 禁止自动停止、删除或覆盖未知服务；
- 禁止把“端口检测通过”误写成“云厂商安全组已放行”。

### 2.3 核心选择

首版：

- REALITY 常驻核心：Xray `v26.3.27`；
- Hysteria2 常驻核心：sing-box `v1.13.14`；
- 客户端配置目标：Mihomo、sing-box、分享链接；
- Mihomo 服务端 listener：仅保留实验扩展接口，不进入生产默认路线。

双核心选择依据：

- 两个入站仍由同一份规范化状态、事务、备份和导出模型管理；
- TCP 443 与 UDP 443 是不同套接字，两核心可共用端口号；
- 实机固定凭据 A/B 测试确认 Xray REALITY 服务端同时兼容 Mihomo、Xray、sing-box 客户端；
- sing-box REALITY 服务端只通过 sing-box 客户端，未达到既定客户端兼容矩阵；
- 以一个额外的低资源服务换取可验证的跨客户端兼容性，并继续禁止面板、数据库和反向代理常驻。

### 2.4 实现栈与运行边界

首版实现锁定为：

- `install.sh`：小型 Bash Bootstrap，只负责下载、验证和安装固定版本的 VPSKit；
- `vpskit`：Go 构建的单文件主程序，负责状态、事务、渲染、升级和卸载；
- `xray`：REALITY TCP 入站核心；
- `sing-box`：Hysteria2 UDP 入站核心，并用于生成 REALITY 密钥和校验 sing-box 客户端配置；
- `lego`：仅签发/续期时运行的 DNS-01 工具；
- systemd：服务和低频 timer；
- 不在目标 VPS 上引入 Python、Node.js、Docker daemon 或动态插件运行时。

Go 构建环境只存在于开发/CI。Bootstrap 和日常管理不得用 Shell 拼接 JSON/YAML；JSON 使用结构化序列化，Mihomo YAML 使用锁定依赖和 golden tests 生成。

## 3. 首版支持范围

### 3.1 操作系统

目标支持矩阵：

- Debian 12；
- Debian 13；
- Ubuntu 24.04；
- systemd；
- amd64；
- arm64。

完成对应 CI/实机证据前，文档只能写“目标支持”，不能对外宣称“已正式支持”。不在首版承诺：

- Alpine/OpenRC；
- CentOS 7；
- Amazon Linux；
- FreeBSD；
- 容器托管环境；
- NAT VPS 无公网入站环境。

原因不是不能实现，而是这些平台会扩大包管理器、服务管理器、防火墙和 CI 测试矩阵，降低首版可靠性。

### 3.2 节点能力

#### VLESS REALITY Vision

首版支持：

- 自动生成 UUID；
- 自动生成 REALITY 密钥对；
- 自动生成 short ID；
- 自定义 SNI/握手目标；
- 自定义 TCP 端口；
- IP 或灰云域名作为连接地址；
- Mihomo、sing-box、VLESS 分享链接导出；
- 配置更新前执行 `sing-box check`。

REALITY 目标站必须经过预检：

- 目标 TCP 443 可达；
- TLS 握手正常；
- 用户填写的 SNI 与目标站匹配；
- 不把某个公共网站永久硬编码为唯一默认值；
- 检查失败时允许用户显式继续，但报告中标记风险。

#### Hysteria2 TLS

首版支持：

- 用户密码；
- 自定义 UDP 端口；
- 有效 TLS 证书；
- 可选 Salamander 混淆；
- 可选带宽参数；
- Mihomo、sing-box、`hysteria2://` 链接导出；
- 清楚区分 TCP 端口检查与 UDP 端口检查。

证书来源与稳定级别：

1. `existing-files`：stable。读取用户已有证书后部署一份受管只读副本，不改变原文件权限；
2. `cloudflare-dns01`：stable。通过固定版本 lego 执行，Token 最小权限为目标 Zone 的 `Zone:Read` 与 `DNS:Edit`；
3. `self-signed-pinned`：advanced。仅在用户明确选择时生成，同时为 Mihomo 和 sing-box 客户端输出证书/公钥指纹；禁止默认写出永久 `skip-cert-verify: true`。

sing-box `v1.13.14` 的内联 ACME 依赖 `with_acme` 构建标签，1.14 又把它迁移到 Certificate Provider。为避免模板和构建标签漂移，VPSKit 不使用 sing-box 内联 ACME 作为首版证书生命周期底座，而由独立 Certificate Provider 管理，再把受管证书路径注入节点配置。

## 4. 用户操作流程

### 4.1 安装 VPSKit

Bootstrap 只做安全编排，节点业务仍由同一份Go应用服务执行：

- 系统、架构和权限检测；
- 下载固定版本归档并校验内嵌 SHA-256；
- 检查归档路径/链接安全，再用内置公钥验证发布清单签名和资产；
- 通过中文向导选择 balanced 或 reality-only、收集必要参数并显示最终计划；
- 用户输入精确 `INSTALL` 后调用签名包内的 `vpskit install` 事务化创建节点；
- 安装完成后执行 doctor 并导出权限为 `0600` 的客户端 ZIP；
- 不自动修改 SSH、BBR、防火墙默认策略或系统内核。

禁止每次执行都下载远程 `main` 分支。上游核心的摘要即使来自同一个 GitHub Release，也只能作为交叉校验；生产信任根是随 VPSKit Release 发布并签名的 `release-manifest.json`。

已有状态存在时Bootstrap拒绝执行初始安装，升级必须走签名包 `vpskit update`。`--archive` 可以使用本地归档，`--verify-only` 只做平台、摘要和签名验证，不修改系统。

### 4.2 环境预检与确认

Bootstrap与应用服务共同检查：

- root 或 sudo 权限；
- OS/架构/systemd；
- 可用磁盘与内存；
- 时间同步；
- DNS 与 GitHub/备用下载源；
- `curl`、`tar`、`sha256sum` 等 Bootstrap 依赖；
- 当前端口监听；
- 当前防火墙后端；
- 是否已存在VPSKit状态或冲突端口。

输出计划后必须输入精确 `INSTALL` 才允许进入节点创建。Cloudflare Token使用隐藏输入并只通过环境传递，不进入命令行参数。

### 4.3 创建节点 Profile

最终用户通过 `install.sh` 选择Profile。下面是签名包内部使用的低级命令，只用于自动化测试、恢复或受控运维。

最低前置条件模式：

```bash
vpskit install reality-only \
  --bundle-dir <signed-bundle> \
  --connect-host <VPS-IP-or-domain> \
  --reality-server-name <verified-domain> \
  --tcp-port 443
```

证书条件满足后的推荐组合：

```bash
VPSKIT_CF_DNS_API_TOKEN='<process-environment-only>' \
vpskit install balanced \
  --bundle-dir <signed-bundle> \
  --domain <certificate-domain> \
  --connect-host <client-host> \
  --reality-server-name <verified-domain> \
  --tcp-port 443 \
  --udp-port 443
```

`balanced` Profile 创建：

- `reality-main`；
- `hy2-backup`；
- 一个 Mihomo `select` 策略组；
- 一个可选 `url-test` 策略组模板；
- 对应的 sing-box 客户端配置和分享链接。

安装过程：

```text
获取事务锁
→ 预检
→ 建立快照
→ 下载并验证签名锁文件、核心和依赖工具
→ 生成临时状态
→ 渲染临时配置
→ 用目标 sing-box 二进制执行 check
→ 生成防火墙变更计划
→ 同文件系统内 fsync + 原子切换配置
→ 受控 restart 服务
→ 验证服务、版本、配置哈希和本地监听
→ 增量应用并验证受支持的防火墙规则
→ 客户端导出一致性检查
→ 提交事务
```

失败时按 ownership 记录逆序回滚，并恢复旧配置、旧二进制和旧服务。sing-box 的 `SIGHUP` 虽会先检查配置再重建实例，但它不能替代 VPSKit 事务：监听创建仍可能在旧实例关闭后失败。因此 v0.1 默认使用受控 restart + 健康检查，不把 reload 成功当成提交依据。

### 4.4 日常命令

```bash
vpskit menu

vpskit instance enable <reality|hysteria2>
vpskit instance disable <reality|hysteria2>
vpskit instance modify <reality|hysteria2> --port <port>
vpskit instance modify reality --reality-server-name <domain>
vpskit instance delete <reality|hysteria2> --yes

vpskit export --format <all|mihomo|sing-box|link|qr|bundle>
vpskit export --format bundle [--output-dir <absolute-directory>]

vpskit reality scan
vpskit status
vpskit doctor
vpskit cert status
vpskit cert renew

vpskit update core --bundle-dir <signed-bundle>
vpskit update self --bundle-dir <signed-bundle>
vpskit rollback <transaction-id> --yes

vpskit backup
vpskit restore <backup-id> --yes
vpskit uninstall --yes
```

安全要求：

- `remove`、`restore`、`uninstall` 必须确认；
- 无头模式必须显式传入 `--yes`；
- 删除前显示将删除的实例、端口和文件；
- 只清理 VPSKit 记录为 owned 的资源。

## 5. 文件与目录

```text
/usr/local/bin/vpskit
/usr/local/lib/vpskit/
├── cli/
├── core/
├── adapters/
├── renderers/
├── providers/
├── migrations/
└── schemas/

/etc/vpskit/
├── vpskit.toml
├── versions.lock
├── instances/
├── profiles/
├── generated/
└── exports/

/var/lib/vpskit/
├── state.json
├── ownership.json
├── secrets/
├── certificates/
├── locks/
├── transactions/
├── backups/
└── cache/

/var/log/vpskit/
├── audit.jsonl
└── health.jsonl
```

核心正式配置放在 VPSKit 自己的生成目录，再由独立 `vpskit-sing-box.service` 指向该目录，避免混入系统已有 `sing-box.service` 或用户手写配置。核心运行日志进入该 unit 的 journald；VPSKit 不再重复写一份 `runtime.log`。

生成后的 sing-box 运行配置会包含认证材料或其受管路径，因此它与导出文件同样属于敏感文件。服务使用专用非 root 用户和组读取 0640 配置；低于 1024 的监听只授予 `CAP_NET_BIND_SERVICE`，不得照搬上游 unit 中与本项目无关的 `CAP_NET_ADMIN`、`CAP_NET_RAW`、`CAP_SYS_PTRACE` 或 `CAP_DAC_READ_SEARCH`。

敏感权限：

| 内容               | 权限建议      |
| ---------------- | --------- |
| 实例状态、导出链接        | 0600      |
| REALITY 私钥       | 0600      |
| TLS 私钥           | 0600      |
| Cloudflare Token | 0600      |
| 生成后的核心配置         | 0640 root:vpskit |
| 普通日志             | 0640      |
| 程序文件             | 0755/0644 |

## 6. 客户端输出

### 6.1 内部单一事实源

服务端配置和所有客户端配置必须从同一份规范化实例状态生成，不能分别拼接。

示例：

```json
{
  "schema_version": 1,
  "id": "reality-main",
  "enabled": true,
  "adapter": "sing-box",
  "protocol": "vless",
  "transport": "tcp",
  "security": "reality",
  "listen": "::",
  "listen_port": 443,
  "connect_host": "node.example.com",
  "users": [
    {
      "name": "default",
      "uuid": "...",
      "flow": "xtls-rprx-vision"
    }
  ],
  "reality": {
    "server_name": "example-target.com",
    "handshake_server": "example-target.com",
    "handshake_port": 443,
    "private_key_ref": "secret://reality-main/private-key",
    "public_key": "...",
    "short_ids": ["..."]
  }
}
```

### 6.2 Mihomo 输出

首版输出：

- 两个 proxy 节点；
- `Proxy` 手动选择组；
- 可选 `Auto` 测速组；
- 私网/LAN 直连；
- `MATCH,Proxy`；
- 不默认塞入大量远程分流和广告规则。

ACL4SSR、Anti-AD 等规则应作为独立 `ruleset pack` 安装，不写死在协议渲染器中。

### 6.3 安全交付与配置修订

初始客户端配置修订号为 `r0001`。启用、禁用、删除实例，修改协议端口或更换 REALITY `serverName` 时必须递增修订号、全量重新生成相关客户端产物，并在命令结果中输出 `client_update_required`、`changed_client_fields` 与回滚备份 ID。核心升级、服务重启和同域名证书续期不改变客户端连接参数，不递增客户端配置修订号。

`vpskit export --format bundle` 生成权限为 `0600` 的 ZIP，内含 Mihomo、sing-box、分享链接、SHA-256 清单与离线说明。默认写入发起 `sudo` 的 SSH 用户主目录并归属该用户，便于通过 SCP/SFTP 下载；显式输出目录必须是已存在的绝对非符号链接目录。ZIP 包含凭据，不进入备份、仓库或公开上传路径，下载并导入后应删除临时副本。

静态 YAML、JSON、分享链接和二维码是配置快照，不能远程自动修改；修订号变化后用户必须重新导出和导入。

### 6.4 订阅服务

首版默认只生成本地文件，不开放 HTTP 订阅端口。

后续订阅服务必须作为独立模块，具备：

- HTTPS；
- 随机长 Token；
- 可吊销；
- `Cache-Control: no-store`；
- 无目录列表；
- 访问日志脱敏；
- 不依赖公开第三方订阅转换器。

## 7. 防火墙原则

禁止：

```bash
iptables -F
iptables -P INPUT ACCEPT
ufw reset
```

正确模型：

1. 检测 nftables/UFW/firewalld 及其是否真正处于 active；
2. v0.1 只把 active UFW、active firewalld 和 `manual/noop` 作为 stable Provider；
3. 原生 nftables 自动写入必须先通过专用 table/chain、优先级和持久化 VM 测试，完成前标记 experimental；
4. 只在受支持 Provider 中增量添加精确的 TCP 或 UDP 规则；
5. 在 `ownership.json` 记录后端、端口、协议和规则标识；
6. 回滚和卸载只删除本事务创建的规则；
7. 如果不能可靠识别现有防火墙，仅输出命令建议，不自动修改；
8. 云安全组永远标记为“需用户确认”。

## 8. 更新策略

### 8.1 版本通道

- `stable`：默认，只允许项目验证过的稳定版本；
- `candidate`：经过 CI 但尚未进入默认；
- `beta`：用户显式启用；
- `pinned`：指定版本；
- `latest`：只用于显示上游信息，不直接生产安装。

### 8.2 更新事务

```text
查询 Release
→ 校验允许通道
→ 下载资产与摘要
→ 临时安装新二进制
→ 用现有配置执行 check
→ 启动临时验证或短暂切换
→ 健康检查
→ 提交
```

保留至少：

- 当前成功版本；
- 上一个成功版本；
- 当前配置；
- 上一个成功配置；
- 对应状态 schema 版本。

### 8.3 `versions.lock` 最小字段

每个资产至少记录：

```yaml
id: sing-box
version: 1.13.14
channel: stable
source_repo: SagerNet/sing-box
source_ref: v1.13.14
source_commit: 25a600db24f7680ad9806ce5427bd0ab8afe1114
asset_name: sing-box-1.13.14-linux-amd64.tar.gz
asset_size: <发布时锁定>
sha256: <发布时锁定>
verified_at: 2026-07-17
template_revision: 1
state_schema_min: 1
```

锁文件必须属于 VPSKit Release、包含签名验证结果，并区分标签对象、解析后的源码提交和二进制资产摘要。更新程序不从默认分支推断版本，也不把 `latest` API 的返回值直接写入生产锁。

## 9. 安全红线

1. 禁止远程 `main` 直接执行；
2. 禁止无摘要校验安装核心；
3. 禁止默认升级全部系统包；
4. 禁止默认修改 SSH；
5. 禁止默认更换内核；
6. 禁止清空防火墙；
7. 禁止覆盖未知配置；
8. 禁止把密钥写入普通日志或 Shell history；
9. 禁止把公网可访问面板作为首版依赖；
10. 禁止安装成功后只输出一句“完成”，必须输出检查矩阵。
11. 禁止从 sing-box 默认 `testing` 或 Mihomo 当前默认 `main` 构建生产资产；
12. 禁止仅凭同源 checksum 认定供应链可信；
13. 禁止 v0.1 动态加载第三方代码插件；
14. 禁止把 `systemctl reload` 返回 0 当成事务成功；
15. 禁止修改外部证书原文件的 owner/mode 来迁就服务权限。

## 10. 安装完成报告

示例：

```text
VPSKit 事务：TX-20260717-001

核心版本与校验：PASS
签名发布清单：PASS
源码 tag/commit：PASS
配置语法检查：PASS
活动配置哈希：PASS
VLESS TCP 监听：PASS 0.0.0.0:443 / [::]:443
Hysteria2 UDP 监听：PASS 0.0.0.0:443 / [::]:443
系统防火墙 TCP：PASS
系统防火墙 UDP：PASS
云厂商安全组：NEEDS_USER_CHECK
TLS 证书有效期：PASS
REALITY 目标站预检：PASS
Mihomo 配置渲染：PASS
sing-box 客户端渲染：PASS
Mihomo compatibility profile：PASS v1.19
敏感文件权限：PASS
回滚点：BK-20260717-001
公网客户端握手：NEEDS_EXTERNAL_CHECK
```

## 11. 明确暂缓但保留接口的功能

首版不加入：

- VMess；
- mKCP/KCP 魔改；
- 旧 QUIC 方案；
- SSR；
- WARP；
- Argo/Cloudflare Tunnel 节点；
- Docker 运行后端；
- Web 面板；
- DD 系统；
- BBR Plus；
- 锐速；
- 建站环境。

这些功能不是全部都推荐以后加入。接口设计的目的，是保证未来评估某项功能时不破坏现有实例，而不是承诺全部实现。详细边界见《02-架构与扩展接口设计.md》。

---

# VPSKit 架构与扩展接口设计

版本：v1.1 源码审核修订版

## 1. 设计目标

扩展接口必须同时满足：

1. 新增协议不修改事务引擎；
2. 新增客户端格式不修改服务端适配器；
3. 新增 Docker/原生运行后端不修改实例业务模型；
4. 新增面板不直接写核心配置；
5. 新增 WARP、Argo 等辅助能力不污染协议字段；
6. 新版本状态结构可以迁移，旧实例可继续运行和回滚；
7. 扩展失败不能导致现有 Reality/Hy2 一起失效。
8. 首版的“扩展接口”是 Go 程序内部契约，不等于允许加载任意第三方动态代码；
9. 所有状态和配置切换必须发生在同一文件系统，支持 `fsync + rename` 和可验证回滚。

## 2. 实现形态

```text
install.sh（薄 Bootstrap）
        ↓
vpskit（Go 单文件 CLI，无常驻控制面）
        ↓
内部编译期接口：Adapter / Provider / Renderer / Backend
        ↓
外部进程：Xray、sing-box、lego、systemctl、受支持的防火墙命令
```

选择 Go 单文件主程序的原因：

- 调用外部程序时使用参数数组，避免大型 Shell 的引用和注入问题；
- 标准库可完成 JSON、哈希、文件锁、原子文件、签名校验和结构化日志；
- 交叉编译 amd64/arm64，目标 VPS 不需要安装语言运行时；
- CLI 执行结束即释放内存，对 1C1G 的常驻开销为 0；
- Renderer 可以使用锁定的 YAML 依赖和 golden tests，不手写脆弱的 heredoc。

v0.1 不使用 Go `plugin`、不扫描插件目录、不从网络下载并执行第三方扩展。未来若开放外部扩展，应采用有版本的子进程协议和签名包，而不是进程内 ABI。

## 3. 分层结构

```text
CLI / TUI
    ↓
Application Service
    ↓
Transaction Engine ───── Ownership Registry
    ↓
Instance Domain Model
    ├── Core Adapter
    ├── Runtime Backend
    ├── Certificate Provider
    ├── Firewall Provider
    ├── Export Renderer
    ├── Ruleset Pack
    └── Optional Feature Module
```

### 3.1 CLI/TUI

只负责：

- 参数解析；
- 交互收集；
- 显示变更计划；
- 调用应用服务；
- 展示结果。

当前 `vpskit menu` 复用统一命令分派器；状态、扫描、导出和变更均回到既有应用服务。菜单自身只读取输入、组织参数和执行精确确认，不复制事务逻辑。

禁止在菜单代码中直接：

- 修改防火墙；
- 写 sing-box JSON；
- 下载核心；
- 执行 systemctl；
- 生成分享链接。

### 3.2 Application Service

将用户意图转换为计划：

```text
create instance
update instance
remove instance
install adapter
update core
render export
repair state
```

应用服务生成 `ChangePlan`，事务引擎执行计划。

### 3.3 Transaction Engine

统一阶段：

```text
PLANNED
→ LOCKED
→ SNAPSHOTTED
→ STAGED
→ VALIDATED
→ APPLIED
→ HEALTHY
→ COMMITTED
```

失败状态：

```text
FAILED
→ ROLLING_BACK
→ ROLLED_BACK
```

事务记录至少包含：

- 事务 ID；
- 发起命令；
- 变更前状态哈希；
- 目标状态哈希；
- 下载资产摘要；
- 创建/修改/删除文件；
- 创建/修改/删除服务；
- 添加的防火墙规则；
- 验证命令和结果；
- 回滚结果；
- 脱敏后的错误信息。

原子写要求：

1. staging、current 与 backup 必须位于同一文件系统；
2. 新文件写完后 `fsync(file)`；
3. 目录切换前后 `fsync(directory)`；
4. 只用原子 `rename` 切换受管 current；
5. `state.json`、`ownership.json`、运行配置和导出清单使用同一个事务 ID；
6. Ctrl-C、SIGTERM、崩溃或断电后，下次运行先检查未提交事务和 artifact mirror。

## 4. 实例模型

### 4.1 核心字段

```json
{
  "schema_version": 1,
  "id": "hy2-backup",
  "display_name": "LA-Hysteria2",
  "enabled": true,
  "adapter": "sing-box",
  "runtime_backend": "native-systemd",
  "protocol": "hysteria2",
  "listen": "::",
  "listen_port": 443,
  "network": "udp",
  "connect_host": "node.example.com",
  "users": [],
  "security": {},
  "transport": {},
  "certificate_ref": "cert://hy2-default",
  "metadata": {
    "created_at": "...",
    "updated_at": "...",
    "managed_by": "vpskit",
    "config_revision": 1,
    "compatibility_profile": "mihomo-1.19"
  }
}
```

协议专有字段放入有版本的扩展对象：

```json
{
  "extensions": {
    "hysteria2.v1": {
      "up_mbps": 20,
      "down_mbps": 100,
      "obfs": {
        "type": "salamander",
        "secret_ref": "secret://hy2-backup/obfs"
      }
    }
  }
}
```

### 4.2 状态与密钥分离

普通状态：

- 端口；
- 协议；
- 公钥；
- SNI；
- 标签；
- 启用状态。

密钥仓库：

- UUID；
- 用户密码；
- REALITY 私钥；
- TLS 私钥；
- Cloudflare Token；
- 订阅 Token。

状态中只保存 `secret://` 引用。首版可以使用权限为 0600 的本地文件密钥仓库；未来可以增加 SOPS、age 或外部 Secret Provider，而不改变实例 schema。

需要明确：渲染后的 sing-box 配置必须包含认证材料或指向私钥，因此它本身也是敏感产物。受管运行配置使用 `0640 root:vpskit`，客户端导出仍使用 `0600 root:root`。日志和事务记录只保存字段名、引用、哈希和脱敏摘要。

## 5. Core Adapter 接口

统一接口示意：

```text
probe()
install(version, signed_asset_ref)
uninstall()
render_server(state) -> staged_files
validate(staged_files) -> validation_result
activate(staged_files)
restart()
reload_capability() -> unsupported|guarded
healthcheck(state) -> health_result
get_version()
export_capabilities()
```

### 5.1 sing-box Adapter

首版实现：

- Hysteria2 inbound；
- 必须用即将启用的目标版本执行 `sing-box check`；
- systemd 原生运行；
- v0.1 固定使用受控 restart + 健康检查 + 失败回滚；
- 不把 `SIGHUP` 当成原子热切换。源码显示它会在 check 后关闭旧实例并重新创建，新监听仍可能失败；
- 版本锁和回滚。

### 5.2 Xray Adapter

当前生产实现：

- 接管 VLESS Reality Vision TCP 入站；
- 使用目标 Xray 的 `run -test` 配置测试命令；
- 独立 systemd 服务；
- 与 sing-box Hysteria2 共存，并允许 TCP/UDP 使用相同数字端口；
- 状态 schema 5 分别保存 `reality_core`、Xray 配置摘要、sing-box 配置摘要与客户端 `config_revision`；schema 4 迁移后从修订 1 开始；
- 配置切换、备份恢复、自更新、卸载和孤儿扫描必须同时覆盖两个核心；
- 禁止在缺少实机兼容性回归时静默更换 REALITY 核心。

### 5.3 外部/历史协议 Adapter

SSR 不应硬塞进 sing-box adapter，因为 sing-box 官方入站列表并不包含 SSR。若未来确有需求，应作为独立外部核心适配器，拥有自己的：

- 二进制来源；
- 配置 schema；
- systemd 单元；
- 安全评估；
- 维护状态和弃用策略。

## 6. Runtime Backend 接口

```text
install_runtime()
render_service(adapter_artifact)
start()
stop()
reload()
status()
collect_metrics()
remove_runtime()
```

### 6.1 native-systemd

首版唯一正式后端：

- 资源最少；
- 网络路径直接；
- 日志和端口易诊断；
- 无 Docker daemon 常驻开销；
- 适合 1C1G。

首版不复用或覆盖发行版/用户已有的 `sing-box.service`，而创建 `vpskit-sing-box.service`：

- `User=vpskit`、`Group=vpskit`；
- 端口小于 1024 时仅保留 `CAP_NET_BIND_SERVICE`；
- 不授予 `CAP_NET_ADMIN`、`CAP_NET_RAW`、`CAP_SYS_PTRACE`、`CAP_DAC_READ_SEARCH`；
- 外部证书由 Provider 部署受管只读副本，不能靠扩大进程权限读取 `/root`；
- unit、运行目录和 timer 都进入 ownership。

### 6.2 docker

后续可选：

- 作为 runtime backend，而不是协议实现；
- 实例状态和客户端渲染保持不变；
- 镜像必须固定 digest；
- 卷挂载必须使用相同的 generated/secret 路径模型；
- host/network/端口映射由后端处理；
- Docker 未安装时不得由协议模块自动安装；
- 不允许从 native 切换 Docker 时丢失回滚点。

这样以后加入 Docker，不需要重写 Reality/Hy2 业务逻辑。

## 7. Export Renderer 接口

```text
supports(instance) -> bool
validate_capability(instance) -> warnings
render(instance_set, options) -> output_files
self_check(output_files) -> result
```

首版渲染器：

- `mihomo-yaml`；
- `sing-box-json`；
- `share-link`；
- `qr-text`；
- `client-zip`：只打包当前 Profile 的客户端文件，包含 SHA-256 清单和离线说明，拒绝覆盖同名目标。

每个客户端 Renderer 必须绑定 compatibility profile。首版至少固定：

- sing-box 客户端：与服务端锁定版本同主次版本；
- Mihomo：以审核过的 `v1.19` 字段集为基线；
- 分享链接：记录 scheme revision，并用 Mihomo/sing-box 转换测试覆盖 Reality、Hy2、指纹、SNI、short ID 和端口跳跃字段。
- 客户端 ZIP：权限固定为 `0600`，默认归属发起 `sudo` 的 SSH 用户；输出目录必须经过绝对路径、目录类型与符号链接检查。

后续：

- Loon；
- Surge；
- Shadowrocket；
- v2rayN custom subscription；
- HTTPS subscription endpoint。

渲染器只能读取实例状态，不能修改服务端配置。

## 8. Certificate Provider 接口

```text
issue(request)
renew(cert_id)
validate(cert_id)
paths(cert_id)
revoke(cert_id)
status(cert_id)
```

Provider 与稳定级别：

1. `existing-files`（stable）：用户已有证书；验证链、域名和私钥匹配后，部署受管副本；
2. `cloudflare-dns01`（stable）：固定 lego 版本和资产摘要，Token 以子进程环境变量注入，不写入命令行；
3. `self-signed-pinned`（advanced）：明确选择的备用模式，必须生成并导出证书/公钥指纹。

设计要求：

- 证书生命周期与 sing-box adapter 解耦；
- 续期成功后先验证证书和私钥匹配，再 reload；
- 续期失败不能覆盖现有可用证书；
- API Token 仅写入密钥仓库；
- ACME 客户端必须固定版本或包来源；
- 不依赖 80/443 空闲完成 DNS-01。
- Cloudflare Token 只授予目标 Zone 的 `Zone:Read` 与 `DNS:Edit`，不接受 Global API Key 作为默认流程；
- lego 仅以 one-shot 方式运行，不常驻；续期任务读取 0600 secret file 后以环境变量传给子进程；
- 不使用 sing-box `v1.13` 内联 ACME 作为生命周期底座，也不要求未来 `v1.14+` Certificate Provider 成为唯一实现。

## 9. Firewall Provider 与 Port Registry

### 9.1 Port Registry

所有监听先登记：

```json
{
  "port": 443,
  "protocol": "tcp",
  "owner": "instance:reality-main",
  "address": "::"
}
```

TCP 443 与 UDP 443可分别登记；TCP 443 与另一个 TCP 443冲突。

### 9.2 Firewall Provider

后端：

- UFW（active 时 stable）；
- firewalld（active 时 stable）；
- nftables（自动写入暂列 experimental）；
- manual/noop。

必须实现：

```text
probe
plan_add
apply_add
verify
rollback
plan_remove
```

每条规则带稳定标识或精确参数，记录 ownership。

不得因为系统存在 `nft` 命令就认定可以安全写规则。原生 nftables Provider 进入 stable 前，必须在 Debian/Ubuntu VM 中验证专用 table/chain、hook priority、现有 drop policy、重启持久化、Docker/UFW 共存和精确卸载；否则只输出建议命令。

## 10. Ruleset Pack 接口

分流规则与节点协议解耦：

```text
rulesets/
├── minimal-direct/
├── acl4ssr/
├── acl4ssr-antiad/
└── custom/
```

安装节点时只生成最小可用配置。用户以后选择 ACL4SSR 或 Anti-AD 时，只替换 Mihomo 输出中的策略和规则部分，不修改 VPS 服务端。

## 11. 可选 Feature Module

统一形式：

```text
feature_manifest
preflight
plan
apply
healthcheck
rollback
remove
```

### 11.1 WARP

定位：出站辅助模块，不是入站协议。

未来接入方式：

- 生成一个 outbound/endpoint；
- 通过路由规则引用；
- 不改 Reality/Hy2 用户和监听；
- 模块故障时可切回 direct；
- 不默认修改全局 IPv4/IPv6 路由。

### 11.2 Argo/Cloudflare Tunnel

定位：传输/入口辅助模块。

未来接入方式：

- 独立 cloudflared runtime；
- 独立凭据；
- 只代理显式兼容的 inbound；
- 不能把 Hysteria2 UDP 直接假设为普通 Cloudflare 代理可承载；
- 不自动占用或改写现有 Tunnel。

### 11.3 Web 面板

定位：VPSKit API 的一个客户端，而不是直接配置编辑器。

正确方式：

```text
Panel → authenticated local API → ChangePlan → Transaction Engine
```

错误方式：

```text
Panel → 直接写 sing-box config.json
```

面板未来加入时：

- 默认只监听 localhost；
- 认证和 TLS 独立；
- 可以完全卸载而不影响节点；
- 不引入必须常驻的数据库，优先读取 VPSKit 状态；
- 面板不可绕过事务和审计。

### 11.4 建站环境

只提供端口协同接口：

- 查询 80/443 占用；
- 提供已有证书路径接入；
- 不自动安装 Nginx/Caddy/PHP/数据库；
- 未来若做建站插件，必须是独立仓库或独立模块。

### 11.5 BBR、BBR Plus、锐速

- 原生 BBR：可作为可选 `system network` 模块，只修改受管 sysctl drop-in；
- BBR Plus/锐速：涉及第三方内核或历史模块，不进入正式插件目录，除非未来重新完成安全、兼容和回滚评估；
- 节点安装不得依赖任何拥塞控制修改。

### 11.6 DD 系统

DD 会破坏整个宿主机状态，与可回滚节点管理器的责任边界冲突。即使未来提供，也应是完全独立的外部工具推荐，不作为 VPSKit 插件。

## 12. 协议扩展分类

| 功能              | 后续扩展位置                             | 对现有实例影响        |
| --------------- | ---------------------------------- | -------------- |
| TUIC、AnyTLS     | sing-box adapter 的 protocol plugin | 不影响已有协议        |
| VMess           | sing-box protocol plugin           | 不影响已有协议，但默认不推荐 |
| mKCP/KCP        | transport plugin                   | 必须显式实验标记       |
| 旧 QUIC          | legacy transport plugin            | 默认禁用、可计划废弃     |
| SSR             | external core adapter              | 独立服务与版本        |
| XHTTP           | Xray/sing-box transport plugin     | 需单独兼容矩阵        |
| Mihomo listener | experimental core adapter          | 不进入生产默认        |

## 13. Schema 迁移

每份状态带 `schema_version`。

迁移规则：

1. 启动时只读识别旧 schema；
2. 生成迁移计划和备份；
3. 新 schema 在临时目录验证；
4. 旧版本程序不得读取未知高版本 schema 并直接写回；
5. 回滚核心不一定回滚状态 schema，因此状态迁移也必须可逆或保留旧副本；
6. 插件字段使用命名空间，如 `hysteria2.v1`，避免字段冲突。

## 14. 未来外部扩展兼容契约

以下契约是 v1.x 之后外部扩展的预留格式，不是 v0.1 的交付物。v0.1 仅实现同等字段的内部注册表，不发现、不下载、不加载外部插件。

每个插件必须声明：

```yaml
id: protocol.anytls
api_version: 1
plugin_version: 0.1.0
requires:
  vpskit: ">=1.2,<2.0"
  adapter: "sing-box"
  core: ">=1.x"
capabilities:
  - server-render
  - mihomo-export
  - singbox-export
risk_level: experimental
```

插件安装前检查契约，不能把未知字段直接写进生产状态。

外部扩展若未来开放，还必须声明包摘要、签名、许可证、子进程协议版本、允许执行的外部命令和卸载清单。禁止使用 Go 进程内 `plugin` 机制作为生产扩展 ABI。

## 15. 失败隔离

- 配置按实例片段生成，最终 merge；
- 新插件先在 staging 目录渲染；
- 全量 `sing-box check` 通过才替换；
- 新入站健康检查失败时回滚整个事务；
- 插件安装失败不能修改现有 binary、配置、服务或防火墙；
- 可选模块使用独立 systemd unit；
- 双核心模式下分别维护适配器状态和回滚点。

## 16. 源码快照与生产依赖边界

`source-references/` 用于设计审核、许可证核验和行为对照，不进入 VPS 安装包，也不作为运行时 import 路径。任何实现代码若参考第三方仓库：

1. 先确认该文件的许可证；
2. GPL/无明确许可证项目默认只学习行为和 UX，不复制代码；
3. MIT/MPL 代码若复用，保留所需版权和许可证通知；
4. 在 `NOTICE`/依赖清单记录来源、commit 和用途；
5. 第三方源码更新不能绕过 VPSKit 自身测试和签名发布流程。

---

# VPSKit 低配资源预算与运行策略

版本：v1.1 源码审核修订版
基准环境：1 vCPU、1 GB RAM、约 10 GB 磁盘。

> 本文数值是项目的工程目标和告警阈值，不是对所有 VPS、线路、连接数和核心版本的固定性能承诺。正式发布前必须在目标机器上实测。

## 1. 低配原则

1. 默认运行一个 Xray REALITY 进程和一个 sing-box Hysteria2 进程；
2. 两核心共享规范化状态、事务和导出，不引入常驻控制面；
3. 不常驻 Docker daemon；
4. 不常驻 Web 面板、数据库、反向代理；
5. 客户端 YAML/JSON 只在变更时生成；
6. VPSKit 主命令为 Go 单文件程序，执行结束即退出；目标 VPS 不安装 Python/Node.js 运行时；
7. 日志默认 `warn`，限制文件大小；
8. 更新缓存和旧备份有明确上限；
9. 不默认开启复杂规则数据库或服务端流量统计；
10. 不默认编译核心，使用官方发布资产。

## 2. 内存预算

### 2.1 工程目标

| 项目                 | 目标       | 告警       | 处理建议            |
| ------------------ | --------:| --------:| --------------- |
| Xray + sing-box 双核心空闲 RSS | ≤ 120 MB | > 180 MB | 分进程检查版本、日志、连接和异常增长 |
| VPSKit 常驻内存        | 0 MB     | 任意常驻     | CLI 运行结束应退出     |
| VPSKit 单次 CLI 峰值      | ≤ 128 MB | > 192 MB | 检查导出规模与异常缓存      |
| 核心更新/解压总峰值          | ≤ 180 MB | > 250 MB | 串行处理，及时清理临时文件    |
| 全机安装后空闲可用内存        | ≥ 350 MB | < 250 MB | 提示关闭面板/探针等服务    |

不要在 systemd 中一开始设置过低的 `MemoryMax`。硬限制可能在并发、QUIC 或升级后导致服务被杀。推荐：

- 默认只启用 `MemoryAccounting=yes`；
- `MemoryHigh`/`MemoryMax` 作为高级可选设置；
- doctor 报告实际 RSS 和峰值；
- 经过实测后再设置，例如 MemoryHigh 192M、MemoryMax 256M，而不是写死所有机器。

### 2.2 Swap

脚本行为：

- 检测当前 swap；
- 没有 swap 时只告警，不自动创建；
- 可提供独立命令 `vpskit system swap create --size 512M`；
- 创建前检查磁盘空间；
- 记录 fstab/文件 ownership；
- 卸载节点不删除 swap；只有显式 system 命令可删除。

10 GB 磁盘下不建议默认创建 1–2 GB 大 swap。512 MB 作为应急选项更合理，但仍需用户选择。

## 3. CPU 预算

### 3.1 空闲目标

- sing-box 空闲平均 CPU：目标低于单核 2%；
- VPSKit 不常驻，因此空闲 CPU 为 0；
- 禁止每分钟轮询上游或重生成订阅；
- 证书检查、版本检查使用 systemd timer，频率按天或周。

### 3.2 负载原则

实际吞吐取决于：

- CPU 型号和虚拟化超售；
- TLS/AEAD 性能；
- 包大小；
- UDP 丢包和 Hysteria2 拥塞控制；
- 并发连接数；
- VPS 带宽上限。

验收不能仅看 Speedtest 峰值。至少记录：

```text
CPU user/system
RSS/peak RSS
TCP retransmissions
UDP errors/drops
连接数
吞吐
延迟与抖动
```

1C 环境下不要同时运行大规模测速、系统更新、压缩备份和核心升级。

## 4. 磁盘预算

### 4.1 目标分配

| 内容                   | 建议上限   |
| -------------------- | ------:|
| VPSKit 程序、模板、schema  | 40 MB  |
| 当前 sing-box 二进制及运行文件 | 80 MB  |
| 上一版本回滚资产             | 80 MB  |
| 当前 lego 与证书工具资产      | 50 MB  |
| 配置与客户端导出             | 10 MB  |
| 事务元数据                | 20 MB  |
| 运行日志                 | 50 MB  |
| 备份总额                 | 200 MB |
| 更新临时空间               | 250 MB |

目标：长期占用尽量控制在 450 MB 左右；更新过程临时峰值控制在 750 MB 以内。具体资产大小应由发布 CI 实际统计。

`source-references/` 是开发机审查资料，不打入 Release、不复制到 VPS，也不计入上述预算。

### 4.2 保留策略

- 核心二进制：当前 + 上一个成功版本；
- 配置备份：最近 5 个成功事务；
- 失败事务：最近 3 个；
- 普通日志：5 × 5 MB 或相近限制；
- 审计日志：10 × 5 MB；
- 下载缓存：成功提交后删除非必要文件；
- ZIP/tar 包：校验和解压成功后删除；
- 超过阈值时先提示，再清理明确可重建缓存，不自动删唯一回滚点。

## 5. 日志策略

默认：

- sing-box 日志级别 `warn`；
- 调试模式临时切换到 `info/debug`，到期自动恢复；
- 敏感字段脱敏；
- sing-box runtime 输出只进入 `vpskit-sing-box.service` 的 journald，不再重复写一份 runtime.log；
- VPSKit 只维护受控大小的 `audit.jsonl` 与 `health.jsonl`；
- 不输出完整订阅链接；
- 运行日志、审计日志、健康报告逻辑分离。

日志分类：

| 日志          | 内容               |
| ----------- | ---------------- |
| journald / `vpskit-sing-box.service` | 运行错误和核心状态 |
| audit.jsonl | 谁在何时执行了什么变更，参数脱敏 |
| health.jsonl | doctor 和周期性健康摘要 |

不建议为 VPSKit 修改全局 journald 上限；若确需修改，必须作为独立系统模块并显示影响范围。

## 6. systemd 策略

建议 unit：

```ini
[Service]
Type=simple
User=vpskit
Group=vpskit
Restart=on-failure
RestartSec=3s
LimitNOFILE=65535
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadOnlyPaths=/etc/vpskit
ReadWritePaths=/var/lib/vpskit/runtime /var/log/vpskit
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
MemoryAccounting=true
TasksMax=256
```

若所有监听端口都不低于 1024，应去掉 `CAP_NET_BIND_SERVICE`。实际沙箱参数必须在目标版本和证书路径下验证。外部证书应由 Provider 复制到受管目录，不能为读取 `/root` 证书而授予 `CAP_DAC_READ_SEARCH`。某些强隔离项可能阻断读取系统证书、DNS 或网络信息，不能不测试就全部开启。

不默认设置：

- CPUQuota；
- MemoryMax；
- OOMScoreAdjust；
- 实时调度；
- CPU 绑核。

## 7. 配置简化

首版服务端配置只包含：

- 日志；
- VLESS Reality inbound；
- Hysteria2 inbound；
- direct/block 基础 outbound；
- 必要路由；
- 必要 TLS/证书路径。

不在服务端加载：

- 大型 GeoIP/GeoSite 规则库；
- ACL4SSR；
- 广告规则；
- Dashboard API；
- 数据库；
- 多用户流量统计；
- 远程 rule-provider 定时更新。

分流主要发生在 Mihomo 客户端，避免把低配 VPS 变成复杂策略网关。

## 8. 证书资源策略

DNS-01 证书工具使用固定版本 lego，只在签发/续期时运行，不常驻。Cloudflare Token 由 0600 secret file 读取并通过子进程环境变量注入，不能出现在 argv、Shell history 或日志中。

续期流程：

```text
检查剩余有效期
→ 启动临时 ACME 客户端
→ DNS-01
→ 写入 staging
→ 校验证书链、域名和私钥匹配
→ 原子替换
→ sing-box check
→ reload/restart
```

续期频率不应每小时执行。每日检查一次到期时间即可，只有进入续期窗口才发起签发。

## 9. 健康检查频率

默认不运行常驻监控 Agent。

建议：

- 每次变更后：完整健康检查；
- 每日：本地服务状态、证书有效期、磁盘空间；
- 每周：上游版本信息检查；
- 不默认从外部每分钟探测节点；
- 用户已有 Komari/哪吒等探针时，VPSKit 不重复安装。

使用 systemd timer 而不是常驻循环。

## 10. 资源验收

### 10.1 空闲验收

在 VPS 启动并稳定 5 分钟后：

```bash
systemctl status vpskit-sing-box.service
ps -o pid,rss,%cpu,cmd -C sing-box
systemd-cgtop
free -m
df -h
ss -ltnup
```

通过条件：

- 两个入站都监听；
- 无持续重启；
- 空闲 RSS 在目标/告警区间内；
- 空闲 CPU 无持续异常；
- 日志没有循环报错；
- 可用内存和磁盘达到阈值。

### 10.1.1 lab31 实机观测

2026-07-20 在 Debian 13 amd64 的 clean install、重启、自更新及客户端验收后观测：

| 进程 | 空闲 RSS 观测 |
| --- | ---: |
| Xray `v26.3.27` REALITY | 约 38–41.5 MB |
| sing-box `v1.13.14` Hysteria2 | 约 45–66.3 MB |
| 合计 | 约 83.5–107.8 MB |

该短时空闲观测低于 120 MB 目标，但不替代 24 小时稳定性、并发连接、下载压力与更新解压峰值测试。

### 10.2 单客户端验收

分别测试：

1. 只使用 Reality；
2. 只使用 Hysteria2；
3. Mihomo 在两节点间切换；
4. TCP 443 与 UDP 443 同端口共存；
5. 重启服务后自动恢复；
6. 更新配置后没有旧进程占端口。

### 10.3 压力与保护

首版不承诺多用户高并发。建议测试：

- 1、5、10 个并发连接档位；
- 长连接和短连接；
- UDP 丢包环境；
- 100 MB/1 GB 文件下载；
- 运行期间执行 doctor；
- 磁盘接近阈值时拒绝升级；
- 内存低于安全线时拒绝并行更新。

## 11. 低配机器的默认开关

默认开启：

- 双核心但无常驻控制面；
- warn 日志；
- 有限备份；
- 版本锁；
- 事务回滚；
- systemd timer 低频检查。

默认关闭：

- Docker；
- 面板；
- 订阅 Web 服务；
- 流量数据库；
- 大型服务端规则；
- Debug 日志；
- 自动每日更新核心；
- 端口跳跃大范围规则；
- WARP/Argo；
- 内核替换。

---

# VPSKit 开发路线与验收标准

版本：v1.1 源码审核修订版

## 1. 开发原则

- 先完成可靠闭环，再增加协议；
- 安装、变更、升级、删除必须共用事务引擎；
- 先验证回滚，再写漂亮菜单；
- 所有客户端输出必须由同一状态模型生成；
- 所有新增功能必须证明不会破坏已有 Reality/Hy2；
- 任何“支持某系统/架构”的声明必须有 CI 或实机证据。
- v0.1 只实现编译期内部扩展接口，不实现第三方动态插件市场；
- 先建立签名发布清单和版本锁，再允许任何在线安装/更新。

## 2. Phase 0：设计冻结

交付物：

- 仓库目录；
- 许可证选择；
- Go 单文件主程序与薄 Bash Bootstrap 的技术决策记录；
- 威胁模型；
- 状态 JSON Schema v1；
- 插件 API v1；
- 事务状态机；
- ownership 模型；
- 版本锁文件格式；
- VPSKit Release 清单签名格式与内置公钥轮换方案；
- 本地参考源码快照清单、许可证清单和 NOTICE 策略；
- 测试矩阵。

验收：

- 协议、运行后端、证书、导出、防火墙已解耦；
- 明确哪些功能永不进入核心模块；
- schema 有正反例；
- 回滚所需数据全部可表示。
- 明确 v0.1 不加载外部插件，预留接口不转化为首版工作量；
- `versions.lock` 能区分上游 tag、解析后的 commit、二进制资产摘要和模板 revision；
- Mihomo、sing-box 等默认分支不能进入生产构建来源。

## 3. Phase 1：基础框架

任务：

1. `vpskit init/status/doctor`；
2. OS/架构/systemd 检测；
3. 互斥锁；
4. 事务目录；
5. ownership registry；
6. 签名发布清单验证、下载器和 SHA-256/大小校验；
7. 同文件系统 `fsync + rename` 原子替换；
8. 备份/回滚；
9. 日志脱敏；
10. native-systemd backend；
11. `vpskit-sing-box.service` 专用用户、组和最小 capability；
12. 未提交事务与 artifact mirror 恢复入口。

破坏性测试：

- 下载中断；
- 磁盘满；
- 校验失败；
- Ctrl-C/SIGTERM；
- 锁文件残留；
- 签名无效、标签对象与 commit 混淆；
- systemctl 启动失败；
- 配置文件只写一半；
- 未知防火墙后端。

通过标准：

- 失败不留下半安装服务；
- 再次运行可识别遗留事务；
- rollback 可恢复变更前哈希；
- 不修改任何未知服务。
- 外部证书原文件 owner/mode 不发生变化；
- 非 root 服务不需要 `CAP_DAC_READ_SEARCH`、`CAP_NET_ADMIN` 或 `CAP_NET_RAW`。

## 4. Phase 2：sing-box Reality

任务：

- sing-box 固定版本安装；
- VLESS Reality Vision schema；
- 密钥生成和 secret store；
- 目标站预检；
- TCP 端口注册；
- server config renderer；
- Mihomo/sing-box/link renderer；
- `sing-box check`；
- 受控 restart、本地监听、活动版本和配置哈希检查。

验收场景：

- TCP 443 空闲；
- TCP 443 被 Nginx 占用；
- IPv4-only；
- IPv6 双栈；
- 无域名，IP 连接；
- 灰云域名连接；
- 错误 private/public key；
- 错误 short ID；
- 错误 SNI；
- 升级后配置不兼容并回滚。
- `SIGHUP` check 成功但新监听启动失败时，事务仍能恢复旧版本。

## 5. Phase 3：sing-box Hysteria2

任务：

- Hysteria2 schema；
- UDP 端口注册；
- existing certificate provider；
- 固定 lego 版本的 Cloudflare DNS-01 provider；
- self-signed-pinned advanced provider；
- 证书校验与续期事务；
- Mihomo/sing-box/link renderer；
- UDP 防火墙规则；
- 云安全组提示。

验收场景：

- 与 Reality 共用数字端口；
- UDP 端口被占用；
- TLS 域名不匹配；
- 证书/私钥不匹配；
- 证书续期失败保留旧证书；
- Cloudflare Token 权限不足；
- Token 未出现在 argv、日志或审计详情；
- existing-files Provider 不修改外部证书权限并正确刷新受管副本；
- 客户端不支持某可选字段时给出警告；
- UDP 不通时不误判为服务启动失败。

## 6. Phase 4：双核心双入站 Profile 与客户端输出

任务：

- `balanced` Profile；
- Xray REALITY 与 sing-box Hysteria2 在一次事务中创建；
- Mihomo 手动组和测速组；
- 最小规则；
- export 一致性校验；
- QR 输出；
- 带修订号、SHA-256 清单和离线说明的 `0600` 客户端 ZIP；
- 修改任一实例后全量重新渲染相关导出。
- REALITY 目标变更必须先通过扫描器端到端验证，并明确返回客户端更新字段和回滚备份 ID；
- 为 Mihomo 和 sing-box 输出绑定 compatibility profile；
- 在 CI 中用固定 Mihomo/sing-box 版本解析或加载生成结果。
- 用固定 Mihomo、Xray、sing-box 客户端分别执行 REALITY 端到端回归。

验收：

- 服务端与四种输出参数一致；
- Mihomo、Xray、sing-box 客户端均能通过 Xray REALITY 服务端；
- 自签证书模式生成指纹固定字段，且默认不出现 `skip-cert-verify: true`；
- 删除 Hy2 后 Reality 仍正常；
- 删除 Reality 后 Hy2 仍正常；
- 同一用户字段变更不会遗留旧链接；
- 导出失败不能回滚已正常运行的服务端变更，除非事务策略明确将导出视为强一致步骤；
- 推荐首版采用强一致：导出失败则整个变更回滚。
- 静态配置修订变化后明确提示重新导出，不把本地文件误表述为可自动更新订阅；
- 客户端 ZIP 不覆盖同名文件，默认可由发起 `sudo` 的 SSH 用户通过 SCP/SFTP 下载。

lab31 验收记录（2026-07-20）：固定版本 sing-box、Mihomo、Xray 自动化握手通过；用户随后在 Clash Verge 与 Hiddify 中分别验证 REALITY、Hysteria2，四项人工客户端验收全部通过。

lab32 验收记录（2026-07-20）：schema 5迁移、无效目标零写入、REALITY目标切换并恢复、原凭据不变、修订号1→3、安全ZIP清单、双协议VPS回环和Windows固定版本解析通过；修订3随后在Clash Verge与Hiddify中完成REALITY、Hysteria2四项重新导入验收。

lab33 最终验收记录（2026-07-20）：先创建受管备份与root-only恢复快照，并在Windows端完成传输SHA-256和归档读回验证；随后在同一Debian 13 amd64 VPS执行受管卸载与固定Bootstrap从零重装。schema 5、balanced、初始修订1、证书、systemd服务、orphan scan、安全ZIP、REALITY与Hysteria2回环均通过；VPS重启后服务、doctor和双协议回环再次通过；新配置最终在Clash Verge与Hiddify完成REALITY、Hysteria2四项公网GUI验收。用户确认后删除远程11项恢复/安装临时材料和本机恢复目录，删除后节点健康，accepted客户端配置保留。

## 7. Phase 5：更新、恢复和卸载

任务：

- stable/candidate/beta/pinned；
- 核心更新；
- 自更新；
- schema migration；
- backup/restore；
- 干净卸载；
- orphan scan；
- 上游资产锁、VPSKit 签名清单和公钥轮换；
- 防回滚策略：默认拒绝安装低于当前安全基线的已撤销版本。

验收：

- latest 不自动安装；
- sing-box `testing` 和 Mihomo 当前 `main` 不能成为生产来源；
- 摘要错误拒绝更新；
- 签名错误、资产大小错误、commit/tag 不匹配均拒绝更新；
- 新核心 check 失败回滚；
- 新核心启动后崩溃回滚；
- 上一版本二进制可恢复；
- 恢复不覆盖用户未受管文件；
- 卸载只删除 ownership 中的资源；
- 证书若由其他服务共用，不自动删除。

## 8. Phase 6：正式发布门槛

### 8.1 CI 矩阵

| OS           | 架构    | 级别     |
| ------------ | ----- | ------ |
| Debian 12    | amd64 | 必须     |
| Debian 13    | amd64 | 必须     |
| Ubuntu 24.04 | amd64 | 必须     |
| Debian 12/13 | arm64 | 必须或云实机 |
| Ubuntu 24.04 | arm64 | 建议     |

CI 内容：

- ShellCheck；
- Go format/vet/static analysis/test；
- JSON Schema；
- Bats/Go 集成测试；
- 模板 golden tests；
- 安装/卸载容器测试；
- systemd 实机或 VM 测试；
- 资产 SHA 校验；
- SBOM；
- Secret scanning；
- 不允许测试访问生产 Token。

容器测试只覆盖文件和命令逻辑，不能替代 systemd、UFW/firewalld/nftables、重启持久化、低端口 capability 和 IPv6 的 VM/云实机测试。

### 8.2 Release

每次发布：

- 固定 tag；
- changelog；
- checksums；
- 签名发布清单与可验证构建证明；
- 兼容矩阵；
- 数据迁移说明；
- 回滚说明；
- 已知问题；
- 不推荐执行 `curl main | bash`；
- SBOM、NOTICE 与依赖许可证清单；
- 上游源码 tag/commit、资产摘要和模板 revision 清单。
- 固定版本Bootstrap内嵌仓库、版本和归档SHA-256，支持离线归档与 `--verify-only`；
- 发布工具以显式Linux `0755/0644` 权限生成归档，不依赖构建宿主的文件模式；
- 受保护Environment只在无密钥质量门通过后开放签名密钥；
- 自动生成构建来源证明，但只创建草稿Release，人工复核后再公开。

## 9. 首版最终验收清单

### 功能

- [x] Xray REALITY + sing-box Hysteria2 双核心双入站；
- [x] Reality-only 最小 Profile；
- [x] VLESS Reality Vision；
- [x] Hysteria2 TLS；
- [x] TCP/UDP 同数字端口；
- [x] Mihomo 导出；
- [x] sing-box 客户端导出；
- [x] 分享链接与二维码；
- [x] 带清单和修订号的安全客户端 ZIP；
- [x] REALITY 目标事务化修改、自动重导出和客户端更新提示；
- [x] 修改、禁用、删除；
- [x] doctor；
- [x] 更新和回滚；
- [x] 备份与恢复；
- [x] 卸载。
- [x] 固定版本中文安装Bootstrap；
- [x] 中文管理菜单与变更确认口令；

### 安全

- [x] 固定版本；
- [x] 签名发布清单；
- [x] SHA-256；
- [x] 无敏感日志；
- [x] 0600 密钥；
- [x] 不清空防火墙；
- [x] 不修改 SSH；
- [x] 不替换内核；
- [x] ownership 精确；
- [x] 中断自动回滚；
- [x] 端口冲突不覆盖服务；
- [x] 专用非 root 服务用户；
- [x] 最小 capability；
- [x] 外部证书权限不被修改；

### 低配

- [x] 无 Docker/面板/数据库；
- [x] VPSKit 无常驻进程；
- [x] 日志有限制；
- [x] 备份有限制；
- [x] 更新临时空间预检；
- [x] 空闲 RSS 和 CPU 报告；
- [ ] 1C1G 实机稳定运行；
- [ ] 10 GB 磁盘不会因日志/备份持续膨胀。

### 扩展

- [ ] Core Adapter API；
- [ ] Runtime Backend API；
- [ ] Renderer API；
- [ ] Certificate Provider API；
- [ ] Firewall Provider API；
- [ ] Feature Module API；
- [ ] Schema migration；
- [ ] 插件兼容契约；
- [ ] v0.1 无第三方动态插件加载器；
- [ ] 实验扩展不能修改生产状态。

## 10. 后续功能加入门槛

任何新功能必须回答：

1. 属于哪个扩展层？
2. 是否需要新增常驻进程？
3. 对 1C1G 的内存/CPU/磁盘影响？
4. 是否修改现有端口或防火墙？
5. 能否独立卸载？
6. 失败时如何回滚？
7. 是否有官方维护和客户端支持？
8. 是否需要迁移 schema？
9. 是否增加新的供应链来源？
10. 是否有完整测试矩阵？

回答不完整，不进入 stable。

---

# VPSKit 参考项目与社区调研

版本：v1.1 源码审核修订版
调研与本地源码快照截止：2026-07-17

## 1. 调研原则

来源优先级：

1. 官方配置文档和官方 Release；
2. 官方安装器；
3. 工程质量较高的一键脚本；
4. 社区 Issue/Discussion；
5. 教程站和论坛经验。

论坛内容用于发现需求和故障模式，不替代官方配置定义。

## 2. 本地源码审核范围

本次把会直接影响架构、安全或发布决策的仓库浅克隆到 `source-references/repos/`。源码目录只用于开发机审核，不进入方案 ZIP，也不部署到 VPS。

| 仓库 | 审核 ref | 实际 commit | 主要用途 | 根许可证 |
| --- | --- | --- | --- | --- |
| SagerNet/sing-box | `v1.13.14` | `25a600db24f7680ad9806ce5427bd0ab8afe1114` | Hysteria2、客户端导出、check、systemd | GPL-3.0-or-later + 名称限制 |
| XTLS/Xray-core | `v26.3.27` | `d2758a023cd7f4174a5a5fa4ff66e487d4342ba0` | REALITY 服务端、`run -test` | MPL-2.0 |
| XTLS/Xray-install | `main` | `e741a4f56d368afbb9e5be3361b40c4552d3710d` | 下载、dgst、systemd 模式 | GPL-3.0 |
| MetaCubeX/mihomo | `v1.19.28` | `cbd11db1e13a75d8e680e0fe7742c95be4cba2be` | 客户端字段、分享链接转换 | GPL-3.0 |
| go-acme/lego | `v5.2.2` | `3d5a6695e027d625bd34334d516d77f578d43f11` | Cloudflare DNS-01 Provider | MIT |
| 247like/linux-ssh-init-sh | `main` | `99c0fde79b25a0fadce51d6d979f80ee2d135ef6` | 锁、回滚、artifact、审计 | MIT |
| 233boy/v2ray | `master` | `707ecf7601ff49f91c2d12dd22b98e8f89588d1c` | CLI 与多配置 UX | GPL-3.0 |
| eooce/ssh_tool | `main` | `0b634c2aa7437cb3d4fd2fb0550f2c8573b499bc` | 菜单分类与反例 | 未发现根 LICENSE |

快照选择原则：官方核心固定非预发布 tag；安装器和工程参考固定当日分支 commit。lego 的 `v5.2.2` 是注释标签，表中记录的是解析后的实际 commit，而不是 tag object。

其余项目只做仓库页面/远程 HEAD 对比，没有完整克隆：

| 仓库 | 默认分支与当日 HEAD | 处理 |
| --- | --- | --- |
| eooce/scripts | `master@c55ab47a...` | 快速安装 UX 样本 |
| fscarmen/sing-box | `main@4f29ea5c...` | 多协议/多客户端对比 |
| mack-a/v2ray-agent | `master@85a2e321...` | 大型多核心方案对比 |
| yonggekkk/sing-box-yg | `main@10300a93...` | 新手交互对比 |
| RayWangQvQ/sing-box-installer | `main@d52d4c46...` | Docker 参数/模板对比 |

## 3. 官方项目

### 3.1 SagerNet/sing-box

链接：

- https://github.com/SagerNet/sing-box
- https://sing-box.sagernet.org/
- https://sing-box.sagernet.org/configuration/inbound/vless/
- https://sing-box.sagernet.org/configuration/inbound/hysteria2/
- https://sing-box.sagernet.org/configuration/shared/tls/
- https://sing-box.sagernet.org/configuration/
- https://sing-box.sagernet.org/installation/package-manager/

确认事实：

- sing-box 支持 VLESS 入站；
- VLESS 用户 flow 支持 `xtls-rprx-vision`；
- TLS 配置包含 REALITY 服务端字段；
- sing-box 支持 Hysteria2 入站且 TLS 为必需字段；
- 一份配置包含多个 inbounds；
- 官方提供 `sing-box check`、`format`、`merge`；
- Linux 包通常包含 systemd 服务；
- 官方支持指定版本安装；
- 1.14 文档已经标记部分内联 ACME 字段迁移/弃用方向。

2026-07-17 Release 核验：

- 最新非预发布版为 `v1.13.14`；
- 最新发布流已到 `v1.14.0-alpha.45`，但属于预发布；
- 仓库默认 HEAD 指向 `testing`，而不是 stable Release；
- `v1.13.14` 源码中内联 ACME 由 `with_acme` build tag 控制；
- 官方 systemd unit 的能力集覆盖 TUN/调试等更多场景，不适合作为本项目最小权限模板直接复制；
- `SIGHUP` 路径会先执行 check，再关闭旧实例并重新创建，仍不是事务式无损热切换。

对方案的影响：

- sing-box 单核心双入站在配置能力上成立，但生产兼容性必须由目标客户端端到端测试决定；
- 证书模块必须独立，避免被某个即将弃用的配置字段锁死；
- 更新前必须使用目标版本执行 check；
- 不使用 beta/alpha 作为默认。
- v0.1 用受控 restart、活动配置哈希和失败回滚，不以 reload 返回码提交事务；
- 生产来源只接受 Release tag + VPSKit 签名锁，不接受默认分支。

### 3.2 XTLS/Xray-core 与 Xray-install

链接：

- https://github.com/XTLS/Xray-core
- https://github.com/XTLS/Xray-install
- https://github.com/XTLS/Xray-core/releases

可借鉴：

- REALITY 的主要参考实现和生态；
- 官方 Release 资产；
- 版本固定、systemd、日志和卸载模式；
- Xray `v26.3.27` 已通过固定凭据 A/B 测试，作为 REALITY 生产 Adapter；sing-box 继续承载 Hysteria2。

注意：

- 2026-07-17 最新发布为预发布 `v26.7.11`，最新非预发布为 `v26.3.27`；
- Xray 发布节奏和实验功能变化较快；
- “最新”不等于适合自动升级；
- XHTTP、Finalmask、mKCP 等不能因为出现在 Release 中就自动进入 stable。
- Xray-install 会下载 Release 同源 `.dgst` 并执行 SHA-256 校验。这能发现传输损坏，但同源摘要不能独立抵抗 Release 账号或资产被整体替换；VPSKit 仍需自己的签名锁文件。

### 3.3 MetaCubeX/mihomo

链接：

- https://github.com/MetaCubeX/mihomo
- https://github.com/MetaCubeX/mihomo/releases
- https://wiki.metacubex.one/

定位：

- 首版只作为客户端配置输出目标；
- 利用其 VLESS Reality、Hysteria2、策略组和规则能力；
- 不在低配 VPS 默认常驻 mihomo；
- Release 资产必须按 tag 和 SHA 固定；
- 客户端配置字段随版本变化，渲染器必须有 compatibility profile。

必须特别记录的分支事实：

- 2026-07-17 默认 `main@008b91bf...` 是一个星穹铁道 Mihomo API 的 Python/Pydantic 项目；
- 代理内核源码位于 `Meta`/Release 线，`v1.19.28` 指向 `Meta@cbd11db...`；
- 这能证明“默认分支不是代理内核源码”，但不足以证明仓库被入侵，文档不得作越过证据的归因；
- v1.19.28 源码确认 VLESS Reality、Hy2 端口跳跃、SNI、`skip-cert-verify` 与证书 `fingerprint` 字段存在，分享链接转换也覆盖 `pinSHA256`。

因此 Renderer 必须按明确版本档案生成和测试；禁止下载 `raw/main` 作为 Mihomo schema 来源。

### 3.4 go-acme/lego

链接：

- https://github.com/go-acme/lego
- https://go-acme.github.io/lego/dns/cloudflare/

源码确认：

- 支持 `CF_DNS_API_TOKEN`/`CLOUDFLARE_DNS_API_TOKEN`；
- Cloudflare 最小权限为目标 Zone 的 `Zone:Read` 与 `DNS:Edit`；
- 可拆分只读 Zone Token 与 DNS Edit Token；
- v5.2.2 Release 流程生成 checksums，并通过 GitHub `actions/attest` 为摘要建立构建证明；
- MIT 许可证适合用作独立工具集成，但仍应保留依赖与 NOTICE 记录。

VPSKit 决策：lego 是外部 one-shot Certificate Provider，不作为库编译进核心，也不常驻。VPSKit Release 锁定其资产、大小、SHA-256 与 attestation 结果，Token 只通过进程环境注入。

## 4. 一键脚本项目

### 4.1 247like/linux-ssh-init-sh

链接：https://github.com/247like/linux-ssh-init-sh

最值得借鉴：

- POSIX/严格模式；
- 托管配置块；
- 自动回滚；
- 防失联检查；
- 无头模式；
- 审计日志和健康报告；
- ownership；
- 持久备份与摘要；
- 互斥锁；
- 异常中断后遗留 artifact 提示；
- 幂等重复执行。

源码复核补充：仓库核心实现仍是一份超过 4000 行的 `init.sh`。它的 rollback、artifact mirror、审计净化和备份摘要值得抽象学习，但不能把“大单文件”本身当成 VPSKit 架构模板。该仓库为 MIT，可在保留许可证的前提下复用小型通用实现；首选仍是重新实现适合 Go 事务引擎的模型。

在 VPSKit 中的映射：

| 原项目           | VPSKit              |
| ------------- | ------------------- |
| sshd 配置校验     | sing-box check      |
| SSH 端口监听      | TCP/UDP 入站监听        |
| 防失联回滚         | 核心与配置回滚             |
| 防火墙 ownership | 节点端口规则 ownership    |
| 托管块           | 受管配置目录/片段           |
| audit/health  | VPSKit audit/health |

不直接照搬：

- SSH 加固不是节点安装默认步骤；
- systemd override 参数必须按代理服务重新验证；
- Debian 13 需要自行建立测试证据。

### 4.2 233boy/v2ray

链接：https://github.com/233boy/v2ray

值得借鉴：

- `add/change/del/info/qr/url/status/test/update` 命令体验；
- 多配置实例管理；
- 核心、脚本、数据和 Caddy 分开更新的思路；
- 配置修复与诊断入口；
- 客户端链接和二维码生成。

社区 Issue 暴露的问题：

- jq/unzip 等依赖安装失败；
- GitHub 下载失败；
- 国内网络代理参数与底层 wget 能力不一致；
- systemd 不存在；
- 核心升级后旧配置无法启动；
- 旧版到新版的迁移复杂。

本地源码还确认：安装/运行下载器使用 `wget --no-check-certificate`，更新路径查询 `releases/latest`，下载核心后没有建立与 VPSKit 等价的签名版本锁。这进一步说明它只能作为 CLI/UX 参考，不能复用供应链实现。

对 VPSKit 的要求：

- 依赖预检必须是可测试的；
- 下载器功能要做集成测试；
- 版本锁和 schema migration 是必需项；
- 不复制大量旧协议矩阵；
- 删除必须确认；
- 公开复用代码前核查 LICENSE。

### 4.3 eooce/ssh_tool

链接：https://github.com/eooce/ssh_tool

项目定位：大型 VPS 工具箱，集成系统、Docker、网络、建站、面板、节点和测试脚本。

值得借鉴：

- 中文菜单；
- 功能分类；
- 多发行版识别；
- 已安装状态展示；
- 常用诊断入口。

不应照搬：

- 数千行大单文件；
- 每次运行远程拉取 main；
- 节点、系统优化、建站和 DD 混在一起；
- 宽泛 chmod；
- 直接执行多层第三方脚本；
- 清空或重置防火墙的操作方式；
- 未经明确告知的外部统计请求。

本地源码证据包括：`ssh_tool.sh` 中存在将 iptables 默认策略设为 ACCEPT 后执行 `iptables -F` 的流程、多个远程 `main` 直执行入口，以及对外部计数 API 的调用；仓库根目录未发现 LICENSE。因此 VPSKit 不复制其代码，只借鉴菜单信息架构。

VPSKit 仅吸收交互层，不以它作为安全和事务底座。

### 4.4 eooce/scripts

链接：https://github.com/eooce/scripts

项目提供 Reality、Hysteria2、TUIC 等短脚本，说明社区偏好：

- 无交互快速安装；
- 通过环境变量指定端口；
- 安装后立即输出节点信息。

对 VPSKit 的启示：

- 保留 headless 参数；
- Profile 可以一条命令创建双节点；
- 但不能因此放弃版本锁、事务、回滚和状态管理。

### 4.5 fscarmen/sing-box

链接：https://github.com/fscarmen/sing-box

可参考：

- 多协议、多客户端输出；
- 非交互参数；
- IPv4/IPv6 场景；
- 升级和卸载菜单。

风险：

- 功能密度高、耦合大；
- 模板容易受上游弃用影响；
- 不适合作为低配首版的直接结构模板；
- 代码复用前必须核查许可证。

### 4.6 mack-a/v2ray-agent

链接：https://github.com/mack-a/v2ray-agent

可参考：

- 多核心管理；
- 证书、备份恢复、订阅和菜单 UX；
- 完整的用户故障处理经验。

不适合首版：

- 依赖和功能过重；
- 证书、Web、协议和核心耦合；
- 对 1C1G 的目标过于庞大；
- 许可证义务需单独评估。

### 4.7 yonggekkk/sing-box-yg

链接：https://github.com/yonggekkk/sing-box-yg

可参考：

- 新手交互；
- 多平台客户端信息；
- 节点组合和快速输出；
- IPv4/IPv6、ARM 场景。

不直接采用：

- 协议组合与脚本自身逻辑绑定；
- 不利于构建稳定插件 API；
- 功能优先于事务隔离。

### 4.8 RayWangQvQ/sing-box-installer

链接：https://github.com/RayWangQvQ/sing-box-installer

可参考：

- 参数与模板分离；
- Docker 部署思路。

VPSKit 决策：

- Docker 只作为后续 runtime backend；
- 首版 native systemd；
- 避免 Docker daemon、网络模式和卷管理增加低配成本。

## 5. 社区与教程来源

### 5.1 NodeSeek

链接：https://www.nodeseek.com/

调研限制：

- 本次自动抓取环境无法稳定直接读取主站完整帖子和所有回复；
- 搜索结果、公开摘要和镜像可以用于发现主题，但不足以代表完整社区共识；
- 因此不把任何单个 NodeSeek 帖子当成技术事实来源。

本轮仍未获得可稳定复核的帖子正文/完整回复，因此以下内容只能保留为此前公开摘要中观察到的需求线索，不标注为“社区共识”：

- 用户希望不装面板，CLI 一键输出节点；
- Reality 与 Hysteria2/TUIC 常组合为 TCP/UDP 互补节点；
- 关注小内存、端口共用、IPv6/NAT VPS 和客户端兼容；
- 对脚本供应链、跑路仓库和远程 main 执行存在担忧；
- 对 mihomo listener 服务端有实验兴趣，但维护和适用性并无统一结论。

VPSKit 处理：

- CLI 为默认；
- Xray REALITY + sing-box Hysteria2 双核心双入站；
- 实验能力与 stable 分离；
- Release/SHA/版本锁；
- NodeSeek 仅作为故障样本补充。

### 5.2 BWGSS

链接：https://www.bwgss.org/

参考文章示例：

- https://www.bwgss.org/6862.html
- https://www.bwgss.org/6452.html
- https://www.bwgss.org/6428.html
- https://www.bwgss.org/6406.html
- https://www.bwgss.org/sing-box

提炼经验：

- Hysteria2 使用 UDP，系统防火墙与云安全组都要确认；
- 域名使用 Cloudflare 时通常应为 DNS Only，而不是普通橙云代理；
- Hysteria2 的 TLS、SNI、密码和客户端字段必须一致；
- 面板适合多用户和频繁管理，一键脚本适合少量固定节点；
- 单节点链接与订阅导入需要按客户端能力区分；
- 新增入站应独立创建，不直接改动正在使用的入站。

2026-06 的 Hy2 教程再次明确了 UDP、安全组、TLS 和客户端版本问题。它属于教程站的操作经验，不是协议规范；例如具体面板字段和默认 ALPN 仍要以目标核心源码/文档为准。

这些经验支持 VPSKit 的：

- UDP 专项诊断；
- 云安全组 `NEEDS_USER_CHECK`；
- 状态单一事实源；
- 无面板默认；
- 事务化新增实例。

### 5.3 V2RaySSR

链接：https://v2rayssr.com/

参考文章：

- https://v2rayssr.com/reality-2.html
- https://v2rayssr.com/hysteria2.html

提炼经验：

- REALITY 常见失败点是目标站、SNI、密钥、short ID 和客户端参数不一致；
- Hysteria2 常见失败点是 UDP、证书、自签验证、SNI、密码和客户端字段；
- “服务启动成功”不能证明公网客户端可用；
- Hysteria2 带宽参数不能盲目写高；
- 客户端支持程度随版本变化。

其 Hy2 文章发布时间较早，并包含无域名时使用 `insecure: true` 的便利做法。VPSKit 不采用这个默认值，而改为有效证书或自签指纹固定。该站的在线订阅转换工具也不会成为 VPSKit 依赖，避免把节点凭据发送给第三方服务。

VPSKit 对应：

- REALITY target preflight；
- 客户端 compatibility profile；
- 服务端、本机、系统防火墙、云安全组、公网客户端分层报告；
- 带宽参数默认保守或留空，不宣传为线路魔法。

### 5.4 JHXIE

链接：https://www.jhxie.com/

内容特点：

- 包含传统 V2Ray、系统初始化、BBR Plus、锐速和旧系统教程；
- 能反映早期用户希望“一条脚本包办系统优化”的习惯；
- 部分内容的系统、内核和协议背景已经老化。

VPSKit 决策：

- 不把 BBR Plus/锐速/换内核作为节点部署默认步骤；
- 原生 BBR 只做独立可选模块；
- 教程中的命令必须按当前系统和官方文档重新验证；
- 旧教程只作为历史故障和用户预期参考。
- 其 BBR Plus 示例仍使用 `wget --no-check-certificate` 执行第三方脚本，进一步支持“系统优化必须与节点部署解耦”的结论。

## 6. 社区共性问题 → 设计要求

| 社区问题           | VPSKit 设计要求                   |
| -------------- | ----------------------------- |
| curl main 直接执行 | 固定 tag、先下载、校验 SHA             |
| 依赖安装失败         | preflight、明确依赖、可离线包           |
| 更新后服务启动失败      | 新版本 check + 回滚                |
| TCP 通但 Hy2 不通  | UDP 专项检查                      |
| 系统端口开了仍不通      | 云安全组单独提示                      |
| 配置改了客户端未同步     | 单一实例状态 + 全量渲染                 |
| 防火墙被脚本重置       | 精确增量规则 + ownership            |
| 面板太重或暴露        | CLI 默认、面板后续可卸载客户端             |
| 多脚本互相覆盖        | Port Registry + 服务探测          |
| 旧协议越堆越多        | stable/legacy/experimental 分级 |
| 1G 内存不够        | 双核心最小服务、无常驻控制面、资源预算         |
| 证书续期破坏服务       | staging 验证 + 原子替换             |
| 默认分支不是发布源码     | 固定 Release tag、commit 与 compatibility profile |
| 同源摘要一起被替换       | VPSKit 自签名 release manifest       |
| reload 后新监听失败      | restart/health/rollback，不信任返回码   |

## 7. 来源使用边界

- 官方文档决定字段和协议能力；
- Release 决定版本、资产和变更风险；
- GitHub Issue 用于构建失败测试集；
- 一键脚本用于借鉴 UX 和工程模式；
- 论坛和教程用于补充真实用户误区；
- 不直接复制未知许可证或高风险代码；
- 正式开发前应为每个参考仓库保存当时的 tag/commit 和 LICENSE 摘要。
- 本地源码快照只作审查证据，不表示授权复制代码；无 LICENSE 仓库默认不得复制；
- 所有社区来源都必须标注证据层级和访问限制。

## 8. 当前上游状态提醒

调研时可见：

- sing-box 稳定与 1.14 alpha 文档/功能并存；默认应选择稳定通道；
- sing-box 文档已经出现 1.14 的新字段和弃用提示，模板必须按目标版本渲染；
- Xray Release 同时包含 stable/pre-release 与快速变化功能，不能盲目 latest；
- Mihomo Release 频繁，客户端渲染需要版本档案；
- Mihomo 当前默认 main 与代理内核 Release 线不同，必须显式使用 tag/Meta；
- 现有一键脚本即使仍维护，也可能保留旧协议、旧系统和历史兼容代码。

因此 `versions.lock + capability matrix + migration` 是本项目的核心，而不是附加功能。

---

# VPSKit 源码审核与修订意见

> 标题：VPSKit 源码审核与修订意见
>
> 生成时间：2026-07-17 11:58
>
> 生成者：Codex
>
> 版本：v1.1
>
> 用途：记录 v1.0 方案的本地源码审核结论、已落实修订和后续开发门槛

## 1. 审核结论

结论：**主路线通过，需按 v1.1 修订版执行。**

保留的核心方向：

1. 低配 VPS 默认只常驻一个 sing-box；
2. Reality 与 Hysteria2 是两个独立入站，可在同一进程中使用 TCP/UDP 同数字端口；
3. Mihomo 只作为客户端配置输出目标；
4. 安装、变更、升级、删除共用事务、ownership 和回滚；
5. Docker、面板、WARP、Argo、内核/SSH 修改与节点部署解耦。

v1.0 不宜直接进入编码的原因，不是协议路线错误，而是若干工程边界仍未锁定：默认分支来源、证书工具、服务权限、供应链信任根、防火墙支持级别、reload 语义、实现语言和插件范围。v1.1 已把这些问题写入 01–05 文档。

## 2. 本地审核材料

完整源码快照位于项目根目录 `source-references/repos/`。本次重点检查：

- sing-box `v1.13.14`：VLESS/Hy2 options、`sing-box check`、SIGHUP reload、官方 systemd unit；
- Xray-core `v26.3.27`：`run -test` 与 REALITY 配置；
- Xray-install：Release `.dgst`/SHA-256、systemd 与卸载逻辑；
- Mihomo `v1.19.28`：VLESS Reality、Hy2、指纹、分享链接转换；
- lego `v5.2.2`：Cloudflare DNS-01、最小 Token 权限、Release checksums/attestation；
- linux-ssh-init-sh：锁、artifact mirror、rollback、审计和备份校验；
- 233boy/v2ray：命令体验、多配置管理和下载/更新反例；
- eooce/ssh_tool：菜单分类及防火墙/远程执行/统计请求反例。

源码仓库不进入发布 ZIP，也不部署到 VPS。

## 3. 必须修订项与处理结果

| 优先级 | 审核发现 | 风险 | v1.1 处理 |
| --- | --- | --- | --- |
| P0 | sing-box 默认 HEAD 是 `testing` | 把预发布代码误当稳定版 | 生产只接受稳定 Release tag + commit + 签名锁 |
| P0 | Mihomo 当前 `main` 不是代理内核源码 | schema/源码来源完全错误 | 固定 Release tag/Meta，并记录 compatibility profile |
| P0 | SHA 与资产同源不足以形成独立信任根 | Release 被整体替换时仍会“校验通过” | VPSKit 自签名 `release-manifest.json` 作为生产锁 |
| P0 | v1.0 未选择 ACME 工具 | 实现时重新耦合到 sing-box 字段 | 固定 lego one-shot Provider；existing-files 与 self-signed-pinned 分级 |
| P0 | 官方 sing-box unit 权限过宽 | 低配自用节点拥有无关能力 | 独立非 root unit，仅按需授予 `CAP_NET_BIND_SERVICE` |
| P1 | SIGHUP 在 check 后仍会关闭并重建实例 | reload 返回不代表新监听健康 | v0.1 使用 restart + health + rollback |
| P1 | 自动 raw nftables 范围过宽 | 与现有规则、UFW/Docker 交互不可预测 | v0.1 stable 仅 active UFW/firewalld/manual；raw nft experimental |
| P1 | “正式支持”没有测试证据 | 文档承诺超过实际状态 | 改成目标支持矩阵，CI/实机通过后才升级声明 |
| P1 | v1.0 插件接口接近首版动态插件框架 | 扩大供应链、ABI 和测试面 | v0.1 只保留 Go 内部编译期接口，不加载第三方插件 |
| P1 | Bash/Python/渲染职责未冻结 | 开发中出现大型 Shell 和运行时依赖 | 薄 Bash Bootstrap + Go 单文件主程序 |
| P2 | balanced 被描述成无条件默认 | 无域名/证书/UDP 时安装失败或诱导 insecure | 增加 Reality-only 最小 Profile；balanced 只在条件满足时推荐 |
| P2 | runtime.log 与 journald 重复 | 1G/10G 机器增加磁盘占用 | 核心日志只进 journald，VPSKit 保留 audit/health JSONL |

## 4. 直接源码证据形成的设计意见

### 4.1 配置能力成立，但生产兼容性要求改用双核心

sing-box `v1.13.14` 的 VLESS inbound 包含用户 UUID/flow/TLS，Hy2 inbound 在创建阶段明确拒绝未启用 TLS 的配置；`check` 会构造完整实例，说明“单 sing-box + 两个独立 inbound”在 schema 和启动层面成立。但后续同一 VPS、同一 REALITY 凭据的 A/B 实测显示：sing-box 客户端可连接 sing-box REALITY 服务端，Mihomo `v1.19.29` 与 Xray `v26.3.27` 客户端均认证失败；当服务端仅替换为 Xray `v26.3.27` 后，三类客户端均成功。因此生产方案改为 Xray 承载 REALITY、sing-box 承载 Hysteria2。

### 4.2 不把官方 systemd unit 原样复制

上游 unit 需要兼顾 TUN、raw socket、调试和外部证书读取，包含多项广泛 capability。VPSKit 的服务器双入站不需要这些能力。正确做法是建立自己的 unit、用户、受管证书目录和权限矩阵，并在 VM 中验证 sandbox。

### 4.3 reload 只能是优化，不能是事务提交点

sing-box SIGHUP 会检查配置、关闭旧实例、再进入创建循环。check 能验证对象构造，但不能证明所有监听在旧实例关闭后必然成功。因此 v0.1 的 correctness path 必须是：切换候选配置、restart、验证服务/版本/配置哈希/监听；失败恢复旧版本并再次验证。

### 4.4 自签证书可以保留，但不能靠 insecure

Mihomo v1.19.28 已有 Hy2 `fingerprint` 和分享链接 `pinSHA256` 转换，sing-box 也有证书公钥 SHA-256 固定能力。自签模式可以作为 advanced fallback，但 Renderer 必须按客户端版本生成指纹字段并测试；默认不输出 `skip-cert-verify: true`。

### 4.5 参考脚本以“概念复用”为主

- linux-ssh-init-sh：MIT，可借鉴 artifact/rollback/audit 机制，但其主体仍是大型单文件，不作为代码结构模板；
- 233boy/v2ray：GPL-3.0，且当前下载器跳过 TLS 校验，只借鉴 CLI 与多配置 UX；
- eooce/ssh_tool：根目录未发现 LICENSE，并存在清空防火墙和远程 main 直执行，只借鉴菜单分类；
- sing-box/Mihomo/Xray-install 等 GPL/MPL 项目不得在未履行许可证义务时复制实现代码。

## 5. 开发前仍需补齐的证据

截至 2026-07-20，原审核缺口已部分转为实机证据：

1. Debian 13 amd64 已运行 lab31 生成的双 systemd unit，并通过卸载重装与重启持久化；Debian 12、Ubuntu 24.04 与 arm64 仍只是目标支持；
2. 双核心短时空闲 RSS 已记录为约 83.5–107.8 MB；24 小时稳定性、并发、下载压力和更新解压峰值仍待测；
3. Cloudflare DNS-01 与 ZeroSSL 颁发链已跑通，lab31 使用 existing-files 复用有效证书并保留续期配置；尚未等待一次真实到期前自动续期触发；
4. lab32 签名开发包已在实机完成 schema 5 迁移、安全 ZIP 导出、无效 REALITY 目标零写入和目标切换恢复，最终双协议回环与Windows固定版本解析通过；修订3已在Clash Verge/Hiddify中完成REALITY、Hysteria2四项重新导入验收，正式 CI 多系统矩阵尚未全部执行；
5. 公网 TCP REALITY 与 UDP Hysteria2 已完成端到端握手，Mihomo、Xray、sing-box 固定客户端已通过隔离链路兼容测试；用户也已在 Clash Verge 与 Hiddify 中分别确认 REALITY、Hysteria2 可用；
6. `manual/noop` 防火墙模式已验证不清空现有规则，但 UFW/firewalld、Docker 与云安全组组合矩阵仍未完成；
7. VPSKit Ed25519 签名清单、三资产版本锁、摘要拒绝和公钥轮换测试已建立；lab33固定Bootstrap、显式Linux权限归档、构建来源证明工作流和草稿Release门已实现并完成本地/实机验证；
8. public GitHub仓库 `filence/vpskit` 已完成首次提交并通过全部CI，首发版本采用 `v0.1.0`；`production-release` Environment、`v0.*` tag限制和新生产签名信任根已经建立，正式tag与草稿Release仍须按门禁推进；
9. 没有做正式法律审查；当前许可证意见仍只是工程合规边界。

因此当前可以表述为“lab31/32客户端与lab33发布入口在一台 Debian 13 amd64 实验 VPS 上通过”，不能扩张为“三种系统和全部架构均已验证”。

验收清理记录（2026-07-20）：删除了 VPS 上约 172 MB 的重复恢复副本和约 57 MB 的旧 lab30 安装包，并删除本机五批过期客户端测试目录；保留 lab31 受管回滚备份、活动 `0600` 凭据和四份最终客户端配置。清理后 doctor、双服务、续期定时器、端口与配置摘要再次通过。

## 6. 推荐实施顺序

1. 建仓库、许可证、Go module、Bootstrap 和目录骨架；
2. 固化 `release-manifest.json`、`versions.lock`、签名验证和 source manifest；
3. 完成 state/schema、secret store、ownership、事务与 crash recovery；
4. 只实现 Reality-only Profile，在 VM 中跑通安装/变更/回滚/卸载；
5. 加入 existing-files 与 self-signed-pinned，再实现 Hy2；
6. 接入 lego Cloudflare DNS-01；
7. 完成 balanced Profile、Mihomo/sing-box 导出与 Xray REALITY Adapter；
8. 最后再评估 raw nftables 和任何外部扩展协议。

## 7. 审核后的最终意见

v1.1 可以作为开发基线。最重要的不是继续增加协议，而是先把下面四条做成可自动验证的事实：

- 每个安装资产都能追溯到 tag、commit、摘要和签名锁；
- 每次变更都有可恢复的旧配置、旧二进制和 ownership；
- 非 root 服务只拥有双入站真正需要的权限；
- 文档中每个“支持”都有 CI 或实机证据。

这四条完成后，Reality + Hy2 的功能开发才不会退化成另一个难以维护的一键脚本合集。
