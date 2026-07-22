# VPSKit 功能演进完整方案 v0.3-R17

> 标题：VPSKit 功能演进完整方案
>
> 生成时间：2026-07-22 09:56
>
> 生成者：Codex
>
> 版本：v0.3-R17
>
> 用途：用户筛选后的 VPSKit 后续功能实施依据

- 原编制日期：2026-07-20
- 精简修订日期：2026-07-21
- 当前产品基线：VPSKit `v0.2.7-lab.1`；方案 A r0008、方案 B r0014、Salamander r0015、schema 9 白名单/自定义规则 r0012、`doctor --fix`、扩展 `system inspect`、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵与系统更新候选检查已完成对应实机验收
- 证据原则：仅保留功能进入实施路线；暂停功能不安排版本号

本文件由同目录 00–12 分卷按顺序机械合并。出现歧义时，以分卷、`FILE-MANIFEST.md` 和当前源码为准。

---

# 00｜执行摘要与决策清单

> 标题：VPSKit 功能演进执行摘要与决策清单
>
> 生成时间：2026-07-21 16:50
>
> 生成者：Codex
>
> 版本：v0.3-R5
>
> 用途：记录用户筛选后的后续功能范围与实施优先级

## 1. 当前基线

已完成的基线不再重复规划：Reality + Hysteria2 部署、单 VPS Workers/KV 自动订阅、Mihomo/Clash Verge 与 v2rayN 基础交付、生产发布、订阅读取令牌轮换和撤销。

后续目标收敛为：**先让现有双协议的客户端交付、规则、诊断和 Hysteria2 更可靠；不扩张为综合 VPS 面板。**

## 2. 本轮保留范围

| 范围 | 保留功能 | 目的 |
| --- | --- | --- |
| 架构 | 通用实例模型、Adapter、Renderer Registry | 解除 Reality/Hy2 固定字段耦合，为既有功能维护和未来扩展留接口。 |
| 渲染 | 结构化 Mihomo YAML、节点元数据 | 稳定生成配置，并显示节点名、地区、提供商和稳定 ID。 |
| 规则 | ACL4SSR、anti-AD、远程更新、DNS | 交付“节点 + 分流 + 去广告 + DNS”一体化 Mihomo 配置。 |
| 运维 | `doctor --fix`、`system inspect` | 自动修复受管对象并汇总可读的机器/服务状态。 |
| Hysteria2 | Salamander、拥塞控制/带宽建议、端口跳跃、UDP 调优 | 优先改善当前性能主节点的适应性和可诊断性。 |
| 系统 | Fail2ban、更新/重启/时间/DNS/IPv6 检查 | 补最小安全与日常维护闭环。 |

## 3. ACL4SSR 与 anti-AD 的确定方案

规则实现以用户提供的 Android Clash Meta 文档为依据，保留两个**明确可切换**的 Mihomo Profile：

- **方案 A（默认）**：ACL4SSR 分流 + anti-AD 去广告 + fake-ip DNS + Sniffer；适合需要 App 域名级去广告的设备。
- **方案 B（兼容回退）**：仅 ACL4SSR 分流 + fake-ip DNS + Sniffer；当 anti-AD 误杀登录、验证码、支付或图片接口时切换。

两种方案共用 ACL4SSR 的 LAN、UnBan、Gemini、Telegram、AI、OpenAI、GitHub、YouTube、ProxyMedia、Bing、OneDrive、Microsoft、Apple、ChinaDomain、ChinaCompanyIp 和 ProxyGFWlist 等规则；方案 A 额外包含 anti-AD。

规则优先级固定为：

```text
用户白名单 / 自定义规则
→ LAN
→ Google / Gemini / YouTube 专项代理
→ UnBan
→ anti-AD（仅方案 A）
→ AI、OpenAI、GitHub、Telegram、媒体等代理
→ 明确直连服务、国内域名和国内 IP
→ MATCH,PROXY
```

客户端按 24 小时 interval 检查规则。`v0.2.2-lab.2` 已完成“下载、限额、SHA-256、受管缓存、Worker 发布、回读”的闭环，r0008 已通过 Clash Verge Rev 实机验收。`v0.2.3-lab.1` 进一步将精确域名白名单和自定义 `DIRECT / PROXY / REJECT` 规则保存为 schema 9 状态，并稳定渲染在 anti-AD 前；Debian 13 已完成添加、删除和 r0012 发布回读。上游分支仍可变化，固定对象是 VPSKit 已下载并发布的 revision，而非上游仓库分支。

## 4. 暂停规划（不排版本）

以下功能从活跃路线移除，除非未来重新由用户选回：Loon/Shadowrocket 新 Renderer、加密快照、少量多 VPS 聚合、WARP、AnyTLS、XHTTP + REALITY、TUIC、通用 BBR、Swap、通用防火墙管理和 Web 面板。

这不是永久否定；只是当前不消耗实现与测试额度。

## 5. 精简后的实施顺序

1. 通用实例/Adapter/Renderer Registry、结构化 Mihomo 和节点元数据；
2. ACL4SSR + anti-AD 的方案 A/B、远程规则修订、fake-ip DNS 与 Sniffer；
3. `doctor --fix` 与 `system inspect`；
4. Hysteria2 混淆、拥塞控制/带宽建议、端口跳跃与 UDP 调优；
5. Fail2ban、系统更新/重启需求、时间同步、DNS/IPv6 健康检查。

截至 2026-07-22 的实施状态：方案 A r0008 与方案 B r0014 均已通过 Clash Verge Rev 的订阅更新、加载、切换与实际连接验收；受管规则缓存与 schema 9 白名单/自定义规则生命周期已在 Debian 13 amd64 通过。`doctor --fix`、扩展后的 `system inspect`、Fail2ban 的“应用 → 删除 → 重新应用”、SSH 白名单添加/删除、规则刷新失败时保留活动缓存/订阅、Hysteria2 只读能力矩阵及只读 `system updates` 已通过实机验收。`v0.2.7-lab.1` 已将默认关闭的 Salamander 事务开关发布为 r0015，并通过 Clash Verge Rev 的更新、切换和实际使用验收。剩余高优先级为真实误杀域名白名单命中、通用实例/Adapter 解耦，以及只读性能基准和 UDP 调优设计。

## 6. 不变的安全原则

- 新模块默认关闭、独立状态、可回滚和可卸载；
- 不把 Cloudflare 管理 Token 放到 VPS；
- Workers KV 只承诺完整旧/新修订的最终一致读取，不宣称全球原子切换；
- 端口跳跃只由 VPSKit 自己管理精确的受管规则，云安全组仍需人工确认；
- anti-AD 只做域名级 REJECT，不使用 MITM、用户 CA、HTTPS 解密或脚本改写；
- 未通过 Windows 11 目标客户端实测的功能不得标记为 stable。

---

# 01｜现状审计与产品边界

> 标题：VPSKit 现状审计与产品边界
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：记录当前源码事实、证据等级、架构缺口和不可跨越的产品边界

## 1. 审查范围

本次以以下材料交叉核验：

- 本地仓库与公开仓库 `filence/vpskit`；
- GitHub `main@22b6457db557b3fc3e4b723784e33d6d55f563fe`；
- 公开 `v0.1.0` Release 和第一阶段验收文档；
- `internal/model`、`internal/render`、`internal/app` 等关键源码；
- 原 v0.3 功能演进方案 00～10；
- 用户给出的完整需求讨论；
- Cloudflare、sing-box、Hysteria2、Mihomo、v2rayN 和 Loon 官方资料。

审查时公开仓库没有未关闭 Issue 或 PR；这只表示当前没有公开排队项，不表示方案已被实现。

## 2. 当前正式基线

VPSKit v0.1.0 已完成：

- Xray：VLESS + TCP/RAW + REALITY + Vision；
- sing-box：Hysteria2 + TLS；
- TCP/443 与 UDP/443 共用数字端口；
- `balanced` 与 `reality-only` 部署；
- Cloudflare DNS-01 证书签发与续期；
- Mihomo、sing-box 客户端配置；
- 分享链接、二维码和客户端 ZIP；
- `status`、`doctor`、`reality scan`；
- 实例启用、禁用、修改和删除；
- 自更新、核心更新、备份、恢复和卸载；
- schema 5、`config_revision` 和客户端更新提示；
- 发布资产摘要、签名、SBOM 与 GitHub attestation；
- Debian 13 amd64 普通用户端到端实机验收。

GitHub 主分支对应 CI 已成功完成 Go/生成配置/Shell/Secrets、amd64/arm64 构建和三个发行版 CLI smoke。CI smoke 不能替代 systemd 云实机。

## 3. 当前源码事实

### 3.1 状态模型

`internal/model/state.go` 当前 schema 为 5，仍直接包含：

- `Core`；
- `RealityCore`；
- `Reality`；
- `Hysteria2`；
- 单一 `ConfigRevision`。

这对双协议首版清晰可靠，但不适合直接追加 AnyTLS/TUIC/XHTTP、多发布目标和独立 ruleset 修订。

### 3.2 应用层

`internal/app/app.go` 约 1484 行；虽然证书、备份、更新、Profile 等逻辑已拆到多个文件，命令分派、安装公共逻辑、预检、系统操作和大量辅助函数仍集中在 `app` 包。

问题不是文件长度本身，而是新增 Publisher、Ruleset、Snapshot 和 Feature Module 时缺少稳定的领域接口，容易让不同生命周期互相耦合。

### 3.3 Renderer

`internal/render/render.go` 当前：

- Xray/sing-box JSON 使用结构化对象再序列化；
- Mihomo YAML 使用字符串拼接；
- 节点名硬编码为 `JP-Reality`、`JP-Hysteria2`；
- 只有 `Proxy` 组和最小局域网规则；
- 没有 YAML 序列化依赖；
- 没有 Renderer capability、schema 或独立版本；
- 分享链接同样硬编码 `JP-*`。

`go.mod` 当前除二维码库外没有 YAML 库。这证实“结构化 Mihomo + 节点元数据”应先于自动订阅。

### 3.4 当前客户端证据

用户已确定首批测试环境：

- 操作系统：Windows 11；
- Clash Verge Rev：`v2.5.2`；
- Mihomo 核心：`v1.19.29`；
- v2rayN：`v7.23.1`。

以上是固定测试基线，不等于自动订阅、规则更新或协议连接已经在这些版本通过验收。

已验证：

- Clash Verge 与 Hiddify 的 REALITY/Hysteria2 人工连接；
- 固定版本 Mihomo、Xray、sing-box 的解析或隔离链路测试。

尚未形成 stable 证据：

- v2rayN 节点自动订阅更新；
- v2rayN 独立路由订阅；
- Loon 完整远端配置；
- Shadowrocket 完整配置和远程规则；
- Android 用户所称“Clash”的具体产品、内核和版本。

## 4. 尚未完成或未充分验证

### 4.1 平台和长期运行

- Debian 12 systemd 云实机；
- Ubuntu 24.04 systemd 云实机；
- arm64 云实机；
- 1C1G 24h/7d 长时间运行；
- 10GB 磁盘长期增长；
- 多 VPS 厂商、防火墙和网络组合；
- 高并发、持续下载、更新解压和 OOM 压力。

### 4.2 后续架构

- 节点元数据与稳定 node ID；
- Renderer Registry；
- Publisher 接口；
- Ruleset Pack；
- 通用实例集合；
- Feature Module 生命周期；
- 可移植加密快照；
- 多 VPS 聚合；
- schema 迁移预演命令。

### 4.3 外部能力

- Workers/KV 订阅服务；
- 订阅读取 Token 轮换和吊销；
- 节点级发布凭据隔离；
- WARP 本地代理与 AI 路由；
- Hysteria2 端口跳跃防火墙 ownership；
- 任何新协议。

## 5. 产品定位

VPSKit 应继续定位为：

> 面向开发者个人、少量 VPS、低资源、可回滚的节点部署、客户端交付与生命周期管理工具。

它可以成为个人 VPS 工具箱，但不应成为公共机场系统或通用服务器控制面。

## 6. 模块边界

| 功能 | 定位 | 默认状态 |
|---|---|---|
| Reality/Hy2 生命周期 | 产品核心 | 按 Profile 启用 |
| Renderer 和静态导出 | 产品核心 | 启用 |
| 自动订阅 Publisher | 客户端交付核心 | 用户配置后启用 |
| 规则与去广告 | 客户端交付模块 | 标准规则可选 |
| 本地备份与恢复 | 产品核心 | 启用 |
| 可移植加密快照 | 生命周期扩展 | 显式导出 |
| 多 VPS 聚合 | 少量节点协同 | 显式启用 |
| WARP | 可选出站模块 | 默认不安装 |
| UFW/BBR/Swap | 可选系统模块 | 默认不修改 |
| Web 面板/数据库 | 暂不加入 | 不适用 |
| Docker Backend | 实验/后置 | 默认不启用 |
| DD/魔改内核 | 不加入 | 不适用 |

## 7. 功能加入门槛

任何新增功能必须回答：

1. 属于哪个领域层和状态对象；
2. 当前核心版本是否真实支持；
3. 哪些客户端版本已经实测；
4. 是否新增常驻进程；
5. 内存、CPU、磁盘峰值和稳定值；
6. 是否修改端口、防火墙、路由或 DNS；
7. 是否可单独禁用、回滚和卸载；
8. 是否需要 schema 迁移及回退；
9. 是否增加供应链、许可证或账号权限；
10. 是否扩大订阅和恢复包的凭据泄露面；
11. 是否有离线兜底；
12. 失败是否保持现有节点可用。

## 8. 个人项目允许的简化与不能省略的事项

允许简化：

- 只支持明确列出的系统和客户端；
- 高级功能只覆盖特定版本；
- 不设计多租户权限和中央数据库；
- 严重故障时允许重装；
- 2～10 台 VPS 使用轻量聚合，不建设远程运维平台。

仍不能省略：

- 密钥和 Token 分权；
- 防止覆盖或删除未知文件；
- 发布与核心版本校验；
- 状态迁移和回滚；
- 订阅撤销和轮换；
- 客户端固定版本验证；
- 规则来源、许可证和摘要；
- 证据等级和已知限制。

---

# 02｜目标架构与扩展模型

> 标题：VPSKit 目标架构与扩展模型
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：定义渐进式扩展边界、状态演进、Renderer/Publisher 契约和迁移规则

## 1. 架构目标

后续架构应支持：

- 新增客户端格式不修改服务端协议 Adapter；
- 新增 Publisher 不修改 Renderer 内容模型；
- 新增 WARP 不污染入站状态；
- 新增系统功能不进入节点卸载的默认范围；
- 多 VPS 节点能聚合到同一客户端订阅；
- 服务端、客户端、规则和发布状态可独立判断变化；
- 新模块故障不影响现有 Reality/Hy2；
- 从 schema 5 渐进迁移，不做一次性全量重写。

## 2. 实施原则：先切缝，再迁移

不建议先设计一个庞大的通用插件框架。正确顺序是：

1. 从当前 `RuntimeValues` 提取节点元数据和客户端产物模型；
2. 让现有 Mihomo、sing-box 和分享链接通过统一 Renderer 接口生成；
3. 接入 static Publisher；
4. 接入 Workers Publisher；
5. 为规则和更多客户端扩展 capability；
6. 在加入第三协议前，将固定 `Reality/Hysteria2` 状态迁移为通用实例集合。

每一步都必须能独立发布、回滚和验收。

## 3. 推荐分层

```text
CLI / 中文菜单
        ↓
Application Service
        ↓
ChangePlan + Transaction Engine
        ↓
Domain State
├── Instance Registry
├── Core Adapter
├── Certificate Provider
├── Firewall Provider
├── Node Metadata
├── Renderer Registry
├── Ruleset Registry
├── Subscription Publisher
├── Backup / Portable Snapshot
└── Optional Feature Module
```

## 4. 推荐代码结构

```text
internal/
├── app/                  # 编排，不保存具体格式逻辑
├── transaction/
├── state/
├── instance/
├── adapter/
│   ├── xray/
│   └── singbox/
├── certificate/
├── firewall/
├── artifact/             # 统一产物与修订清单
├── renderer/
│   ├── mihomo/
│   ├── singbox/
│   ├── links/
│   ├── v2rayn/
│   ├── loon/
│   └── shadowrocket/
├── subscription/
│   ├── static/
│   └── workers/
├── ruleset/
├── snapshot/
├── diagnostics/
├── systemmodule/
└── feature/
    └── warp/
```

目录只是目标边界，不要求一次性搬迁全部现有代码。

## 5. 节点元数据先行

在完整实例迁移前，先给现有双协议补充稳定元数据：

```json
{
  "node_id": "jp-01",
  "display_name": "Personal-JP-01",
  "provider": "provider-code",
  "country": "JP",
  "city": "Tokyo",
  "priority": 100,
  "tags": ["personal", "performance"],
  "enabled_in_subscription": true
}
```

规则：

- `node_id` 创建后默认不因显示名变化而改变；
- Provider、地区和备注是客户端元数据，不进入协议认证；
- 用户可修改 `display_name`，但 Renderer 最终节点名必须唯一；
- 不把真实 IP、UUID 或订阅 Token 编入 node ID。

## 6. 通用实例模型

第三协议之前，逐步迁移为：

```json
{
  "schema_version": 7,
  "instances": [
    {
      "id": "jp-01-reality",
      "node_id": "jp-01",
      "protocol": "vless",
      "adapter": "xray",
      "enabled": true,
      "listen": {
        "network": "tcp",
        "address": "::",
        "port": 443
      },
      "connect_host": "node.example.com",
      "extensions": {
        "vless_reality.v1": {
          "server_name": "www.example.com",
          "flow": "xtls-rprx-vision",
          "uuid_ref": "secrets/instances.json#reality_uuid",
          "private_key_ref": "secrets/instances.json#reality_private_key",
          "public_key": "...",
          "short_id": "..."
        }
      }
    }
  ]
}
```

不要在 schema 6 同时完成所有迁移。建议：

- schema 6：节点元数据、Renderer/Publisher 状态；
- schema 7：`instances[]`，同时保留 schema 5/6 的只读迁移；
- schema 7 稳定后才增加第三协议。

## 7. 修订与摘要模型

原方案四个独立计数器容易产生“计数递增但内容未变”或相互漂移。建议保留少量语义修订并以摘要为最终事实：

```json
{
  "state_revision": 19,
  "client_revision": 18,
  "ruleset_revision": 7,
  "artifacts": [
    {
      "target": "mihomo",
      "sha256": "...",
      "renderer_version": 2
    }
  ]
}
```

- `state_revision`：任何受管状态提交；
- `client_revision`：客户端节点、组、DNS 或引用变化；
- `ruleset_revision`：规则内容或来源锁变化；
- `artifact.sha256`：判断具体产物是否改变；
- 发布清单自身使用不可变 `publication_id`，不再额外维护易漂移的全局 `subscription_revision`。

现有 `config_revision` 在迁移时映射为 `client_revision`，不得重置为 1。

## 8. Artifact Set

Renderer 不直接上传。它输出统一产物集：

```json
{
  "schema_version": 1,
  "publication_id": "pub-20260721-000018",
  "node_id": "jp-01",
  "client_revision": 18,
  "ruleset_revision": 7,
  "generated_at": "...",
  "artifacts": [
    {
      "target": "mihomo",
      "media_type": "text/yaml; charset=utf-8",
      "sha256": "...",
      "sensitive": true
    }
  ]
}
```

Publisher 只能接收已经完成 Renderer 校验的 Artifact Set。

## 9. Renderer Registry

每个 Renderer 声明：

```text
name
renderer_version
compatibility_profile
supported_protocols
minimum_tested_client_version
supports_full_config
supports_node_subscription
supports_remote_rules
supports_ipv6
supports_port_hopping
supports_warp_group
validation_command
```

运行流程：

```text
读取状态和节点元数据
→ 读取锁定规则包
→ 生成 Artifact Set
→ 语法/结构校验
→ 固定版本客户端解析
→ 比较敏感字段白名单
→ 计算摘要
→ 交给 Publisher
```

Loon/Shadowrocket 若没有可自动化的官方解析器，必须保留 golden test，并以真实设备导入、刷新和规则命中作为 stable 门。

## 10. Subscription Publisher

最小接口：

```text
preflight(target)
plan(artifact_set)
publish(artifact_set)
readback(publication_id)
activate(publication_id)
rollback(previous_publication_id)
rotate_read_token()
revoke_read_token()
healthcheck()
```

首批实现顺序：

1. static：写入本地 staging，校验后原子替换；
2. workers：调用受认证发布入口，不直接把 Cloudflare 管理 Token 放入 VPS；
3. 内置 HTTPS 服务只作为后期备选。

## 11. Feature Module

统一生命周期：

```text
probe
preflight
plan
apply
healthcheck
disable
rollback
remove
```

WARP、BBR、Swap、Fail2ban 可共享生命周期，但 ownership 和卸载策略必须分别定义。系统模块不能因为节点卸载而自动删除。

## 12. 迁移策略

推荐命令：

```bash
vpskit migrate check
vpskit migrate plan
vpskit migrate apply
```

要求：

- v0.1.0 schema 5 必须可迁移；
- `check` 和 `plan` 零写入；
- 输出旧字段到新字段的明确映射；
- 更新前备份旧状态、旧程序和当前导出；
- 新状态写入 staging；
- 新旧 Renderer 对同一输入做差异审计；
- 所有服务端配置和客户端产物验证后才提交；
- 失败恢复旧状态、旧二进制、旧服务和旧导出；
- 不允许更新程序后才首次发现 schema 无法读取；
- 不允许降级程序静默重写更新 schema。

---

# 03｜自动订阅与多客户端交付

> 标题：VPSKit 自动订阅与多客户端交付方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：定义单 VPS 订阅 MVP、Cloudflare 发布安全、多客户端分级和最终一致性边界

## 1. 基本判断

Cloudflare 准备性检查已由用户于 2026-07-21 确认完成，后续已在同日完成实机部署和验收：`sub-dev.<zone>` 预发布与 `sub.<zone>` 正式环境各使用独立 Worker、KV namespace 和 Custom Domain。两者健康检查通过；VPS 已发布 Mihomo、v2rayN、manifest，并完成 Windows 11 Clash Verge Rev/v2rayN 的 r0003→r0004 订阅更新实测。真实域名、凭据和订阅 URL 不写入方案文档。

`vless://` 和 `hysteria2://` 单节点链接只能承载节点参数，不能完整携带策略组、DNS、分流和广告规则。

因此 VPSKit 需要区分：

1. **节点订阅**：适合 v2rayN 等客户端；
2. **完整配置订阅**：适合 Mihomo/Loon/Shadowrocket；
3. **公共版本化规则资源**：不包含节点凭据；
4. **静态离线导出**：订阅故障时兜底。

## 2. 实施顺序

### Phase A：本地产物闭环

先完成：

- 结构化 Mihomo；
- 节点元数据；
- Artifact Set；
- static Publisher；
- 本地回滚；
- 固定版本解析。

本阶段不需要 Cloudflare 账号即可开发和测试。

### Phase B：单 VPS 订阅 MVP

只开放：

```text
https://sub.example.com/s/<read-token>/mihomo
https://sub.example.com/s/<read-token>/v2rayn
https://sub.example.com/s/<read-token>/manifest
```

先不开放 `auto`，避免不稳定 User-Agent 和缓存导致返回错误格式。

### Phase C：规则和更多客户端

在 MVP 稳定后增加：

```text
/loon
/shadowrocket
/rules/<pack>/<revision>/<name>
```

### Phase D：多 VPS 聚合

增加节点级发布入口、聚合构建和故障节点禁用，不建设中央远程运维面板。

## 3. 推荐 Cloudflare 架构

```text
                 管理面（不在 VPS 常驻）
本地/受保护 CI ── Cloudflare 管理 Token ──> 部署 Worker / KV / Custom Domain

                 发布面
VPSKit ── node_id + 节点级发布凭据 ──> Worker /api/v1/nodes/<node_id>/publish
Worker ── KV Binding ──> 只写该 node_id 的受控 key

                 交付面
客户端 ── read-token ──> Worker /s/<read-token>/<target>
Worker ── KV Binding ──> 返回已激活完整产物
```

三类凭据必须分离：

| 凭据 | 用途 | 存放位置 | 泄露影响 |
|---|---|---|---|
| Cloudflare 管理 Token | 部署 Worker/KV/Custom Domain | 本地凭据库或受保护 CI | 可修改 Cloudflare 资源，最高风险 |
| 节点级发布凭据 | 某台 VPS 发布自己的 node_id | VPS `0600` secret file | 只能替换该 node_id 的候选内容 |
| 订阅读取 Token | 客户端 GET 订阅 | 各客户端 | 可读取节点凭据，不能发布 |

不得让每台 VPS 直接持有 Workers KV Storage Write 管理 Token。Cloudflare 该权限是账户级资源权限，无法天然限制到单个 KV key。

## 4. Worker 路由与方法

### 4.1 公开交付入口

只允许：

- `GET`；
- `HEAD`。

其他方法返回 `405`。

### 4.2 受认证发布入口

只允许：

- `POST /api/v1/nodes/<node_id>/publish`；
- 可选 `POST /api/v1/nodes/<node_id>/revoke`。

要求：

- 节点级随机凭据或 HMAC；
- 常量时间比较；
- 请求时间窗和 nonce/replay 防护；
- body 大小上限；
- schema、node_id、revision 和摘要校验；
- 同一 node_id 串行化；
- 响应和日志不回显凭据或完整节点配置。

发布入口与交付入口可以在同一个 Worker 内实现，也可以分成两个 Worker；安全模型必须保持分离。

## 5. Workers KV 一致性边界

Workers KV 适合“低频写、高频读”的配置分发，但它是最终一致性存储。官方文档明确说明，其他地区可能在缓存 TTL 内继续看到旧值，某些情况下超过 60 秒。

因此不得使用“全局原子切换”表述。正确语义：

1. Renderer 在本地完成全部校验；
2. Worker 将单个目标的完整 bytes、摘要和元数据写成一个对象；
3. 保留不可变历史修订；
4. 激活当前目标对象；
5. 客户端可能暂时看到旧的完整修订或新的完整修订；
6. 发布方回读并等待收敛，超时标记 `DEGRADED`；
7. 回滚是重新激活上一完整修订，同样存在传播窗口。

若未来需要严格的写后读或跨对象事务，应评估 Durable Objects；首版没有必要为个人低频发布引入它。

## 6. KV 数据模型

示例：

```text
node/<node_id>/candidate/<revision>
node/<node_id>/active
artifact/mihomo/revision/<revision>
artifact/v2rayn/revision/<revision>
artifact/mihomo/current
artifact/v2rayn/current
rules/<pack>/<revision>/<name>
audit/<date>/<event_id>
```

规则：

- `candidate` 校验失败不得覆盖 `active`；
- `current` 对象包含完整产物，不依赖客户端再拼多个私密片段；
- 历史只保留有限修订；
- audit 只记录 node ID、修订、摘要、状态和脱敏错误；
- 不允许列举有效 read-token 或节点秘密。

## 7. 订阅读取 Token

- 至少 256 位随机值；
- 服务端只保存哈希或不可逆校验值；
- 不与 Cloudflare API Token、发布凭据复用；
- 支持当前/下一 Token 短暂重叠；
- 支持立即吊销；
- 不在普通日志、错误消息和诊断包中出现；
- Token 泄露等价于节点凭据泄露，轮换后应评估是否同时轮换协议凭据。

多数客户端无法稳定附加自定义 Authorization Header，所以首版使用不可猜测路径 Token。必须明确：路径 Token 仍可能进入客户端历史、系统日志或 Cloudflare 请求日志；需要关闭不必要的请求日志、使用脱敏采样，并定期轮换。

## 8. HTTP 行为

建议响应：

- 正确 `Content-Type`；
- `ETag`；
- `Last-Modified`；
- `X-VPSKit-Client-Revision`；
- `X-VPSKit-Ruleset-Revision`；
- `X-VPSKit-Renderer-Version`；
- `Cache-Control: private, no-store`。

条件请求：

- `If-None-Match` 命中返回 `304`；
- `HEAD` 与 `GET` 头部一致；
- 无效 Token 统一返回 `404` 或无细节 `401`，不要泄露路径是否存在；
- 错误响应不得包含 node ID 列表、内部 key 或有效修订内容。

## 9. Mihomo / Clash Verge / Android Mihomo

首批固定客户端为 Windows 11 上的 Clash Verge Rev `v2.5.2`，Mihomo 核心 `v1.19.29`。所有结构化 YAML、Provider、DNS、策略组和更新行为先以该组合验收；后续核心或客户端升级视为新的兼容矩阵条目，不能自动继承结论。

Mihomo 是首个 stable 完整配置目标，包含：

- 节点；
- 手动、性能优先和稳定优先策略组；
- rule-providers；
- rules；
- DNS 基础策略；
- 节点和规则修订；
- Renderer 版本注释。

推荐组：

```text
PROXY
PERFORMANCE
STABLE
AI
DIRECT
REJECT
```

Android 用户必须确认具体客户端和内核。名称为“Clash”的 Android 应用存在多个停更或分叉版本，不能只凭产品俗称声明支持。

## 10. v2rayN

首批固定客户端为 Windows 11 上的 v2rayN `v7.23.1`。Windows 结论仅绑定该版本；Android 版另行记录和验收。

v2rayN 首版只把节点订阅标为 stable 目标：

```text
/v2rayn
```

内容为 VLESS/Hysteria2 URI 列表或兼容 Base64 包装。

路由规则另行提供：

```text
/v2rayn-routing
```

但需注意：v2rayN 的节点订阅、路由导入和 sing-box rule-set 是不同 UI/数据路径。必须按 Windows/Android 的目标版本分别验证首次导入、URL 更新、规则顺序和核心切换，不能承诺一条节点订阅自动覆盖路由设置。

## 11. Loon

Loon 官方 Scheme 支持远端配置、节点列表、规则和插件的独立导入，也支持更新所有订阅资源。

目标入口：

```text
/loon
/loon-nodes
/loon-rules
```

先标记 experimental，满足以下条件后升级 stable：

- 用户实际版本导入完整配置；
- Reality/Hy2 均连接成功；
- 节点刷新成功；
- 远程规则刷新成功；
- 广告/AI/国内直连命中；
- Token 轮换后可恢复更新。

## 12. Shadowrocket

目标入口：

```text
/shadowrocket
```

Shadowrocket 缺少与开源项目同等级、可由 CI 固定的官方解析器和公开 schema，因此不得仅凭类似 Surge/Loon 的语法推断兼容。Renderer 必须绑定用户实际 App 版本，以实机导入、更新、规则命中和回滚结果决定 stable。

## 13. 更新触发与发布流程

以下变化触发重新生成：

- 节点端口、地址、凭据或 REALITY 目标；
- 节点启停、名称、地区和标签；
- 规则包、白名单、去广告级别；
- DNS 策略；
- WARP/AI 出站策略；
- Renderer 或 compatibility profile；
- 多 VPS 节点清单。

流程：

```text
获取本机发布锁
→ 读取当前状态
→ 生成 Artifact Set
→ 本地语法和固定版本校验
→ 计算摘要并生成 publication_id
→ 调用节点级发布入口
→ Worker 验证并写候选
→ Worker 激活完整产物
→ 本地和远端回读
→ 记录 COMMITTED / DEGRADED / FAILED
```

## 14. 备用与降级

- 始终保留本地静态 YAML/JSON/分享链接/ZIP；
- 订阅服务故障不得停止 Xray 或 sing-box；
- 发布失败不得递增已提交的客户端修订；
- Worker 不可用时客户端继续使用旧缓存；
- 至少保留上一完整发布；
- 内置 HTTPS 服务不作为近期默认方案，它会增加公网端口、证书和常驻进程。

## 15. 多 VPS 聚合

每台 VPS 只发布自己的签名/认证 Node Manifest：

```json
{
  "node_id": "jp-01",
  "node_revision": 10,
  "enabled": true,
  "artifacts_sha256": "...",
  "published_at": "..."
}
```

聚合器：

- 验证 node ID 与发布凭据绑定；
- 拒绝节点修改其他 node ID；
- 只聚合已激活节点；
- 为客户端重新生成唯一节点名和策略组；
- 单节点失败可保持其上一有效修订或显式禁用；
- 不保存 SSH 密码，不远程执行 VPS 命令。

---

# 04｜分流规则与去广告方案

> 标题：VPSKit ACL4SSR 与 anti-AD 规则方案
>
> 生成时间：2026-07-21 16:20
>
> 生成者：Codex
>
> 版本：v0.3-R15
>
> 用途：定义保留的 Mihomo 分流、去广告、DNS、更新和回退能力

## 1. 范围

本卷仅定义 Mihomo/Clash 系客户端的规则交付。规则不写入 Xray 或 sing-box 入站，不改变 Reality/Hysteria2 凭据，也不重启 VPS 上的代理服务。

固定采用 ACL4SSR 作为分流来源、anti-AD 作为域名级去广告来源，并使用用户提供的 Android Clash Meta 方案 A/B 的优先级、DNS 和回退思路。

## 2. 两个可选 Profile

| Profile | 内容 | 适用场景 |
| --- | --- | --- |
| `acl4ssr-antiad`（方案 A，默认） | ACL4SSR + anti-AD + fake-ip DNS + Sniffer | 已完成 Clash Verge Rev r0008 受管规则更新、切换与实际连接验收。 |
| `acl4ssr`（方案 B） | ACL4SSR + fake-ip DNS + Sniffer | 已完成 Clash Verge Rev r0014 受管规则更新、切换与实际使用验收。 |

Profile 是订阅主配置的选择，不是单条节点链接的属性。切换 A/B 后客户端刷新主订阅；仅刷新 Rule Provider 不会切换 Profile 结构。

## 3. 规则集合与顺序

两种方案共享以下 ACL4SSR 集合：

```text
LocalAreaNetwork、UnBan、Gemini、SteamCN、Telegram、AI、OpenAi、Github、
YouTube、ProxyMedia、Bing、OneDrive、Microsoft、Apple、ChinaDomain、
ChinaCompanyIp、ProxyGFWlist
```

为保持用户现有“Google 全业务代理”习惯，额外使用 Google Antigravity、Google 域名集和 Google IP 集；Gemini、YouTube 与 Google 总规则位于 UnBan 前面。

方案 A 再增加 `anti-AD`，完整命中顺序为：

```text
用户白名单 / 用户自定义规则
→ LocalAreaNetwork
→ Google Antigravity、Gemini、YouTube、Google 域名/IP
→ UnBan
→ anti-AD → REJECT
→ Telegram、AI、OpenAI、GitHub、媒体、Bing → PROXY
→ OneDrive、Microsoft、Apple、SteamCN → DIRECT
→ ChinaDomain、ChinaCompanyIp、GEOIP(CN) → DIRECT
→ ProxyGFWlist → PROXY
→ MATCH → PROXY
```

Google 专项规则高于 anti-AD 是刻意选择：Google 旗下广告或统计域名可能走代理而非被拒绝，以“Google 全业务代理”优先。

## 4. DNS 与 Sniffer

两个 Profile 都包含：

- fake-ip DNS；
- bootstrap DNS、直连 DNS、代理 DoH DNS；
- Google/Gemini 规则集使用代理 DNS，国内域名使用国内 DNS；
- `rule-providers` 经 `PROXY` 下载，避免直连 GitHub Raw 失败；
- Sniffer 用于辅助将 IP 连接还原成域名；
- LAN、`localhost`、`*.local`、`*.lan` 和系统连通性探测域名排除 fake-ip。

具体 Mihomo 字段以已验证的目标核心版本为准；不把 Android 专用 TUN 参数写进订阅配置。

## 5. 更新、来源与回滚

`v0.2.2-lab.2` 已实现受管缓存。`vpskit rules refresh --yes` 由 VPS 下载 ACL4SSR、anti-AD 和 MetaCubeX Google 候选源，对每个源施加 1 MiB 上限并记录 SHA-256、字节数、来源 URL、创建时间和 ruleset revision。只有全部源成功后，Mihomo 主配置才改为引用读取 Token 保护的 Worker `/rules/<name>` 地址。

发布链路：

```text
VPS 下载候选规则 → 每源限额与 SHA-256 检查 → 写入不可变缓存 revision
→ 渲染主配置引用 Worker 规则地址 → 作为一套订阅修订发布 → 全部 target 远端回读
→ 客户端更新主订阅并按原有 interval 使用本地缓存。
```

上游 URL 本身仍可能变化，因此“固定”的对象是 VPSKit 已下载、已哈希并已发布的 revision，而不是上游仓库分支。Worker 保留历史 target；订阅回滚会恢复同一修订中的主配置与规则工件。刷新使用临时目录，只有全部源、摘要和 manifest 成功后才原子激活；失败时清除临时目录，保持活动缓存、订阅与客户端配置不变。r0008 已完成 20 个规则 target 的发布、回读和 Windows 11 Clash Verge Rev 实际连接验收；Debian 13 已模拟上游不可达并验证失败保护。

## 6. 白名单与误杀处理

`v0.2.3-lab.1` 已将用户规则保存到 schema 9 状态，并在 Mihomo 的远程 Provider 规则前稳定渲染。白名单仅允许精确 `DOMAIN,DIRECT`，避免一条宽泛例外放开整类域名：

```bash
vpskit rules whitelist add --domain captcha.example.com --yes
vpskit rules whitelist remove --domain captcha.example.com --yes
```

自定义规则支持 `domain`、`domain-suffix`、`ip-cidr` 与 `direct`、`proxy`、`reject`；输入会规范化，重复或同目标冲突规则会被拒绝，`vpskit rules custom check` 会报告规则范围重叠。每次变更创建事务备份、递增 `config_revision` 并自动发布；不递增受管缓存的 `ruleset revision`，以保持已哈希缓存的引用正确。

方案 A 发生登录、验证码、支付、图片或 App 启动异常时：先从客户端日志找出 `anti-AD → REJECT` 的域名，添加精确白名单；若误杀频繁，切换方案 B，不以关闭全部分流作为处理方式。

## 7. 能力边界与验收

anti-AD 可处理第三方广告、追踪、统计和部分启动广告域名；不能可靠处理 YouTube 内嵌广告、正常内容同域广告、服务端插入内容或需要 HTTPS 解密的广告。

首个 stable 验收：

- 方案 A 的 r0008 已显示并使用受管规则地址；服务器端已回读 20 个远程规则工件；
- schema 8→9、白名单/自定义规则添加和删除、最终空规则状态以及 r0012 全目标回读已在 Debian 13 amd64 通过；
- 方案 B 显示 19 个 Provider，不含 anti-AD，并已完成 r0014 客户端更新、切换与实际使用验收；
- ACL4SSR 与 anti-AD 可在 24 小时周期外手动刷新；
- Google/Gemini/AI/GitHub/Telegram 命中代理，国内域名/IP 命中直连，未知流量命中 `MATCH,PROXY`；
- 真实误杀域名白名单命中仍需在 Clash Verge 当前 Mihomo 版本验证；上游失败的服务端缓存/订阅保护已在 Debian 13 完成模拟验收。

不在本次精简范围：Loon/Shadowrocket Renderer、v2rayN 独立路由产物、MITM、HTTPS 解密和脚本去广告。

---

# 05｜WARP 与 AI 出站方案（暂停）

> 标题：VPSKit WARP 与 AI 出站方案
>
> 生成时间：2026-07-21 15:30
>
> 生成者：Codex
>
> 版本：v0.3-R2
>
> 用途：保留被暂停功能的边界，避免其混入当前实现范围

WARP 与 AI 精确出站不在本轮保留范围，不安排版本号、不创建常驻进程、不增加规则出口，也不承诺 AI 服务可访问性。

若未来重新启用，必须独立评估：官方 WARP 客户端资源占用、`ai-only` 与 `custom` 路由、direct/WARP 对比、故障回退、订阅 Renderer 兼容性和隐私选择。它不能改善客户端到 VPS 的 Reality/Hysteria2 链路。

---

# 06｜恢复与多 VPS（暂停）

> 标题：VPSKit 恢复与多 VPS 方案
>
> 生成时间：2026-07-21 15:30
>
> 生成者：Codex
>
> 版本：v0.3-R2
>
> 用途：保留被暂停功能的边界，避免其混入当前实现范围

加密可移植快照、全新/恢复部署模式、2～10 台 VPS 聚合、节点冲突处理和聚合策略组均从当前路线暂停。

`doctor --fix`、`system inspect` 与诊断能力没有取消，已迁入第 07 卷的最小运维范围。现有本地 backup/restore 保持原有能力，但不在本轮扩展为跨机器快照。

---

# 07｜Hysteria2 与最小系统运维功能

> 标题：VPSKit Hysteria2 与最小系统运维方案
>
> 生成时间：2026-07-21 16:50
>
> 生成者：Codex
>
> 版本：v0.3-R16
>
> 用途：定义保留的 Hysteria2 强化、诊断、安全和系统健康能力

## 1. 范围与原则

仅保留用户已选择的 Hysteria2 强化、`system inspect`、`doctor --fix`、Fail2ban 和系统健康检查。所有写操作采用 `status → plan → apply → verify → rollback`，不修改未知配置，不替换内核，不自动重启。

暂停：通用 BBR 管理、Swap、通用 UFW/firewalld 管理、第三方内核、新协议和 Web 面板。

## 2. `vpskit system inspect`

已实现并在 Debian 13 amd64 实机通过。当前只读输出：OS、架构、内核、内存、根磁盘使用量、拥塞控制、默认 qdisc、Xray/sing-box/证书 timer 状态、Reality TCP 与 Hysteria2 UDP 监听、时间同步、DNS 服务器与解析、IPv4/IPv6 全局地址和默认路由、重启需求。

它是所有高级功能的前置检查，不自动修改系统。

尚未实现：CPU/inode、Xray/sing-box RSS、UDP buffer 和网络错误计数；这些不能在当前版本中宣称已经输出。Fail2ban 状态和只读系统更新候选已经实现。

## 3. `doctor --fix`

已实现并在 Debian 13 amd64 实机通过。仅修复 VPSKit 自己可证明安全的问题：缺失或权限不正确的受管运行目录、systemd daemon-reload、校验和与核心配置校验均通过时未启用的 Xray/sing-box/证书续期 timer，以及未完成且具备既有恢复备份的 VPSKit 事务。

如果状态记录的配置校验和与磁盘实际配置不一致，或 Xray/sing-box 的配置校验失败，`doctor --fix` 会拒绝继续，而不会把未知配置拉起。当前不清理缓存，也不修改其他目录或权限。

不得修改 SSH、未知服务、用户手写防火墙、系统包、内核或用户配置。

## 4. Hysteria2 强化

### 4.0 2026-07-22 官方兼容性核验与只读基线

实机运行的 sing-box 是 `1.13.14`，目标 Windows 客户端是 Clash Verge Rev v2.5.2 / Mihomo `1.19.29`。官方 sing-box Hysteria2 入站和对应 `v1.13.14` 源码确认 Salamander `obfs` 可用；Mihomo `1.19.29` 文档与源码也支持 Salamander。因此 Salamander 可作为下一项独立、默认关闭的实验切片。

Gecko 和 `bbr_profile` 虽已被 Mihomo 侧解析，但 sing-box 官方文档将其标为 `1.14.0` 起提供，当前锁定入站不可生成。Mihomo 支持客户端端口范围和 `hop-interval`，但当前 sing-box 入站不拥有端口范围监听；不能把 Hysteria 官方服务端的自动 redirect 方案直接套入 VPSKit。端口跳跃需独立受管 NAT redirect、云安全组、冲突检查和完整回滚，当前阻止实施。

`vpskit hysteria2 inspect` 已在 Debian 13 amd64 实机通过：Hysteria2 正在 UDP/443 监听，`rmem_max`/`wmem_max` 均为 `212992` 字节，Salamander 标为 `EXPERIMENTAL`，Gecko、`bbr_profile` 和端口跳跃均标为 `BLOCKED`。该命令只读，不修改状态、订阅或代理服务。

参考：<https://sing-box.sagernet.org/configuration/inbound/hysteria2/>、<https://wiki.metacubex.one/config/proxies/hysteria2/>、<https://v2.hysteria.network/docs/advanced/Port-Hopping/>。

### 4.1 Salamander 混淆（已完成 r0015 验收）

`v0.2.7-lab.1` 已实现 `vpskit hysteria2 salamander plan|enable --yes|disable --yes`。默认关闭；启用前生成独立强密码，事务更新 sing-box 入站、Mihomo 配置、sing-box 客户端 JSON 和分享链接。状态仅保存混淆类型和 secret 引用，命令输出、审计与方案文档均不回显密码。

Debian 13 amd64 已完成签名包升级、只读 `plan`、启用、受管配置校验、Xray/sing-box health、UDP/443 监听、`doctor` 和订阅 r0015 回读；Windows 11 Clash Verge Rev v2.5.2 / Mihomo `1.19.29` 已完成既有订阅更新、Hysteria2 节点切换与实际使用验收。`disable --yes` 保留为可回滚路径；密码不匹配或不支持的客户端仍会表现为超时，故不会自动启用。

### 4.2 拥塞控制与带宽建议

当前只提供读取与建议，不发布 `bbr_profile` 字段。后续应记录 direct/Reality/Hy2 的 RTT、吞吐、丢包、CPU 和 RSS 对比，并根据实测提出带宽候选值；不得依据一次延迟测试自动改参数，且须保留恢复默认。

### 4.3 端口跳跃

客户端可使用明确的 UDP 端口范围和跳跃间隔。当前 sing-box 入站仍监听单个受管端口，由 VPSKit 的**专用 Hysteria2 redirect 组件**管理精确 IPv4/IPv6 重定向；它不是通用防火墙模块。

启用前必须显示端口范围、现有规则影响、云安全组需开放的 UDP 范围和回滚动作。卸载只删除 VPSKit 创建的该组件规则，不给 sing-box `CAP_NET_ADMIN`。

### 4.4 UDP 调优

读取 socket buffer、`net.core.rmem_max/wmem_max` 和丢包计数，根据内存与目标吞吐生成受管 sysctl 建议；可选应用并记录原值。禁止套用不受控的大型“优化模板”。

## 5. Fail2ban

已实现并在 Debian 13 amd64 实机通过。命令为：

```bash
vpskit security fail2ban status
vpskit security fail2ban plan
vpskit security fail2ban apply --yes
vpskit security fail2ban remove --yes
vpskit security fail2ban whitelist list
vpskit security fail2ban whitelist add --cidr <IP-or-CIDR> --yes
vpskit security fail2ban whitelist remove --cidr <IP-or-CIDR> --yes
```

VPSKit 只管理 `/etc/fail2ban/jail.d/vpskit-sshd.conf` 这个覆盖文件：使用 systemd journal、Debian 内置 `sshd` jail、当前有效 SSH 端口、`maxretry=5`、`findtime=10m`、`bantime=1h`。它不创建第二个 sshd jail，避免与 Debian 默认 jail 争用 nftables 资源；不修改 `sshd_config`、不管理 Reality/Hy2 日志，也不删除 Fail2ban 软件包。

所有权记录保存覆盖文件摘要和最多 32 条规范化白名单。配置文件被手工修改或记录缺失时，应用、删除与白名单变更均拒绝覆盖。白名单只接受单个 IP 或 CIDR；它写入 `ignoreip`，不会修改 SSH 配置、代理日志或任何未知 jail。每次变更先执行 `fail2ban-client -d`，重启后最多轮询 15 秒确认 `sshd` jail 控制 socket 就绪；失败会自动恢复原覆盖文件并重启 Fail2ban。实机已验证“应用 → 删除覆盖文件并保留软件包 → 重新应用”以及“添加保留地址 → jail active → 删除 → 空状态”完整生命周期。

## 6. 系统健康检查

已实现：`reboot-required`、时间同步、DNS 可用性、IPv4/IPv6 配置与默认路由、磁盘使用量提示和 `vpskit system updates`。更新检查调用 `apt-get -s upgrade`，只列出候选包，绝不下载、安装、删除或重启。磁盘增长趋势仍待实现。默认只报告；系统更新、重启和 SSH 安全改动始终由用户单独执行。

## 7. 发布门

- 每项 Hysteria2 功能绑定当前锁定 sing-box 版本与目标客户端实测；
- 端口跳跃在启用、重启、回滚和卸载后均验证端口范围不残留；
- UDP 调优在 1C1G 条件下验证内存余量和恢复原值；
- `doctor --fix` 不得触碰非 VPSKit 文件；
- Fail2ban 只对已声明日志来源生效，并有覆盖文件所有权、应用/删除/重新应用及白名单添加/删除测试；
- `system inspect` 在无 root 写权限时仍可输出安全的只读报告。

---

# 08｜精简版路线图与优先级

> 标题：VPSKit 精简版版本路线图
>
> 生成时间：2026-07-21 16:50
>
> 生成者：Codex
>
> 版本：v0.3-R16
>
> 用途：将用户筛选后的功能拆成低风险、可验收的版本切片

## 1. 已完成

`v0.2.0-lab.1` 已完成单 VPS 自动订阅、Mihomo/Clash Verge 与 v2rayN 基础交付、生产 Workers/KV 发布、令牌轮换/撤销和用户客户端自动更新验收。

`v0.2.1-lab.7` 已完成结构化 Mihomo、节点元数据、方案 A/B 的服务端渲染、方案 A Windows 11 Clash Verge Rev r0007 验收、`doctor --fix`、DNS/IPv4/IPv6 扩展 `system inspect`、Fail2ban SSH jail 完整生命周期以及只读系统更新候选检查。

`v0.2.2-lab.2` 已完成 ACL4SSR/anti-AD 受管缓存和方案 A r0008 实机验收；`v0.2.3-lab.1` 已完成 schema 9 的精确白名单与自定义规则生命周期，并通过 Debian 13 的添加、删除、r0012 发布回读和代理服务回归。方案 B（不含 anti-AD）的 r0014 已通过 Clash Verge Rev 更新、切换和实际使用验收；`v0.2.4-lab.2` 已完成 Fail2ban SSH 白名单添加/删除、jail active 回读和代理服务回归；`v0.2.5-lab.1` 已完成规则源失败时的原子缓存保护实机验收；`v0.2.6-lab.1` 已完成 Hysteria2 能力矩阵与 UDP buffer 只读实机验收；`v0.2.7-lab.1` 已完成 Salamander 默认关闭开关、r0015 自动订阅发布和 Windows 11 Clash Verge Rev 实机验收。

## 2. 下一个版本：架构、渲染与规则交付

目标：不增加协议和 VPS 数量，建立后续维护所需接口，并交付方案 A/B。

- 完成真实误杀域名白名单命中验收；
- 完成通用实例模型与 Adapter 的实际解耦（当前只完成 Renderer Registry 侧的扩展基础）。

## 3. Hysteria2 版本：现有性能主节点强化

- 仅只读的拥塞控制与带宽建议；
- RTT/吞吐/丢包/CPU/RSS 基准；
- 端口跳跃及专用 redirect；
- UDP buffer 检查与可回滚调优。

每项单独开关，先在当前锁定 sing-box 版本和目标客户端验证；不因上游文档存在字段就提前开放。

## 4. 暂停清单

以下不再安排版本号：加密快照、多 VPS 聚合、Loon/Shadowrocket Renderer、WARP、新协议（AnyTLS/XHTTP/TUIC）、通用 BBR、Swap、通用防火墙管理、Docker、Web 面板和多租户功能。

重新启用任何一项前，必须由用户再次选择，并单独评估资源、客户端兼容性、回滚和验收成本。

## 5. 统一发布原则

- 一个版本只处理一个主要风险域；
- 先有状态、回滚、测试，再开放菜单；
- 规则来源和核心版本固定、可追溯；
- 未经过目标客户端实测的能力只标 experimental；
- 当前 Reality/Hy2 与已上线订阅始终是最高回归门。

---

# 09｜精简范围验收与发布门禁

> 标题：VPSKit 精简范围验收与发布门禁
>
> 生成时间：2026-07-21 16:35
>
> 生成者：Codex
>
> 版本：v0.3-R16
>
> 用途：限定当前保留功能的测试证据和发布条件

## 1. 通用门禁

- 固定版本构建、单元测试、配置解析和敏感信息扫描通过；
- 当前 Reality、Hysteria2、生产订阅和静态导出不回归；
- 每项写操作均有 plan、回读与 rollback；
- Windows 11 Clash Verge Rev/Mihomo 与 v2rayN 作为首批客户端门禁。

## 2. 架构、Renderer 与元数据

- 旧状态可迁移且可回退；
- Renderer 产物经固定 Mihomo 版本解析；
- 节点 ID、显示名、地区、提供商变更仅影响预期产物；
- 订阅 revision、ETag 和旧 revision 回读保持正确。

## 3. ACL4SSR 与 anti-AD

- 方案 A 有 20 个 Provider，含 anti-AD；方案 B 有 19 个 Provider，不含 anti-AD；
- fake-ip DNS、Sniffer、代理下载 Provider 和 24 小时更新在目标 Mihomo 验证；
- Google/Gemini/AI/GitHub/Telegram 代理，国内域名/IP 直连，未知流量 `MATCH,PROXY`；
- 方案 B 切换与上游失败保留活动缓存/订阅均已实测；真实 anti-AD 命中后的白名单修复仍待实测；
- 不测试或宣称 YouTube 内嵌广告、MITM 或 HTTPS 解密效果。

## 4. 运维与系统健康

- `doctor --fix` 仅改变 VPSKit 受管对象；
- `system inspect` 在低资源机器输出内存、磁盘、服务、UDP、DNS、时间、IPv4/IPv6 与重启需求；
- Fail2ban 仅对 SSH systemd journal 封禁，覆盖文件所有权、应用、删除、重新应用以及白名单添加/删除均回读；白名单失败路径必须恢复原覆盖文件和服务；
- 更新检查只报告，不自动升级或重启。

## 5. Hysteria2

- `hysteria2 inspect` 必须只读，并报告锁定 sing-box、UDP 监听、UDP buffer 与明确的能力阻止原因；
- Salamander 已完成服务端、订阅 r0015 与 Clash Verge Rev/Mihomo 实测；后续拥塞控制参数和分享/订阅字段仍须在服务端及目标客户端同时验证；
- 端口跳跃验证端口范围、IPv4/IPv6 redirect、重启、云安全组提示与精确卸载；
- UDP 调优在 1C1G 条件下验证资源余量、吞吐/丢包影响和原值恢复；
- 性能报告同时记录 RTT、吞吐、丢包、CPU、RSS，不以单次延迟决定配置。

暂停功能不得通过“顺手实现”进入 Release；重新启用必须先增补专属验收门禁。

---

# 10｜参考项目与资料

> 标题：VPSKit 参考项目与资料
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：记录方案判断所依据的一手资料、版本边界和证据缺口

## 1. 使用原则

1. 优先官方文档、源码、Release 和公共 CI；
2. 所有上游能力必须映射到 VPSKit 锁定版本，不能拿最新版文档替代当前实现；
3. 第三方规则仓库只能作为数据源，必须记录固定版本、许可证和摘要；
4. 参考其他一键脚本时只借鉴功能场景，不继承其不透明下载、全局改机、魔改内核或不可回滚架构；
5. 客户端兼容性以固定版本真实导入和连接结果为准。

本文件资料核验日期：2026-07-21。

## 2. VPSKit 与当前基线

- 仓库：<https://github.com/filence/vpskit>
- 审查提交：`22b6457db557b3fc3e4b723784e33d6d55f563fe`
- 主分支 CI：<https://github.com/filence/vpskit/actions/runs/29729609546>
- 当前状态 schema：5
- 当前核心锁定版本：Xray `v26.6.8`、sing-box `v1.13.14`、lego `v4.31.0`
- 当前实机证据：Debian 13 amd64；其他系统、arm64 和长期低资源运行仍为待验证边界

审查源码时应重点查看：

- `internal/model/state.go`：固定 Reality/Hysteria2 状态模型；
- `internal/app/app.go`：应用层集中处理多类生命周期；
- `internal/render/render.go`：当前 Mihomo 手工拼接、固定节点名与最小规则；
- `internal/install/versions.go`：上游锁定版本；
- `PHASE1_ACCEPTANCE.md`：第一阶段实机证据与已知边界。

## 3. Cloudflare Workers 与 KV

### 一致性与数据模型

- Workers KV 工作原理：<https://developers.cloudflare.com/kv/concepts/how-kv-works/>
- KV FAQ：<https://developers.cloudflare.com/kv/reference/faq/>

关键约束：KV 为最终一致，缓存导致其他地区可能在约一分钟或更久后看到更新；不提供跨 key 原子事务。因此方案采用不可变修订 artifact + 最后发布 manifest，并接受短时间读取完整旧修订。

### 权限与部署

- API Token 权限：<https://developers.cloudflare.com/fundamentals/api/reference/permissions/>
- API Token 模板：<https://developers.cloudflare.com/fundamentals/api/reference/template/>
- Worker 自定义域名：<https://developers.cloudflare.com/workers/configuration/routing/custom-domains/>

关键约束：`Workers KV Storage Write` 是账户级权限，不能据此宣称每台 VPS 拥有天然的 KV key 级写入隔离。VPS 节点应调用自建发布入口，由发布层校验 `node_id`；Cloudflare 管理/部署 Token 只保留在可信管理环境。

## 4. Mihomo

- Rule Providers：<https://wiki.metacubex.one/en/config/rule-providers/>
- 配置文档入口：<https://wiki.metacubex.one/en/config/>

可用能力包括 HTTP/file/inline Provider、刷新间隔、domain/ipcidr/classical behavior 以及 yaml/text/mrs 格式。MRS 只适用于相应 behavior，Renderer 必须按固定 Mihomo 核心进行解析测试。

## 5. sing-box 与 Hysteria2

- Hysteria2 inbound：<https://sing-box.sagernet.org/configuration/inbound/hysteria2/>
- Hysteria2 outbound：<https://sing-box.sagernet.org/configuration/outbound/hysteria2/>
- AnyTLS inbound：<https://sing-box.sagernet.org/configuration/inbound/anytls/>
- Releases：<https://github.com/SagerNet/sing-box/releases>

版本边界：

| 能力 | 上游版本信息 | 对当前 `v1.13.14` 的结论 |
|---|---|---|
| Hysteria2 outbound `server_ports`、`hop_interval` | 1.11 起 | 可用于支持该字段的客户端输出，但仍需实测 |
| AnyTLS | 1.12 起 | 上游具备；VPSKit 未实现 |
| Gecko、`bbr_profile`、Realm、部分跳跃增强 | 1.14 文档/变更 | 当前锁定版本不能直接启用 |

官方 Hysteria 2 端口跳跃说明：<https://v2.hysteria.network/docs/advanced/Port-Hopping/>。该文档中的服务端端口范围与自动防火墙能力属于 Hysteria 官方实现，不能直接等同于 VPSKit 当前 sing-box inbound；本方案选择 VPSKit 管理防火墙重定向，避免给代理核心授予 `CAP_NET_ADMIN`。

## 6. Cloudflare WARP

- Linux 安装与支持范围：<https://developers.cloudflare.com/warp-client/get-started/linux/>
- WARP 模式：<https://developers.cloudflare.com/warp-client/warp-modes/>

WARP 在本方案中只作为可选精确出站。正式实现前必须固定 `cloudflare-warp` 版本，探测 Linux CLI 的实际代理模式、监听地址、DNS 行为、服务依赖和资源占用。它不等于 AI 解锁，也不保证固定国家或更高速度。

## 7. 客户端资料

### v2rayN

- 订阅说明：<https://github.com/2dust/v2rayN/wiki/Description-of-subscription>
- UI 与路由说明：<https://github.com/2dust/v2rayN/wiki/Description-of-some-ui>

普通节点订阅与路由规则是两个不同层面。方案不能把 ACL/广告规则塞进单个 `vless://` 或 `hysteria2://` 节点链接。

### Loon

- URL Scheme：<https://nsloon.app/docs/Scheme/>

Loon 区分远程配置、节点列表、规则和插件入口。正式支持仍需记录 iOS 与 Loon 固定版本，并在真实设备上验证导入、更新和命中。

### Shadowrocket

本次未获得足够稳定、可机器核验的官方完整配置 schema。任何生成格式在真实设备验证前只能标为 experimental，不能因为与 Surge/Loon 格式相似就宣称兼容。

### Android “Clash”

“Clash”不是足够精确的测试对象。前期材料必须提供具体应用名、包名、版本和内置核心；不同分支对 Hysteria2、Provider 和字段支持可能不同。

## 8. 规则数据源与许可证

候选源包括：

- MetaCubeX/meta-rules-dat：<https://github.com/MetaCubeX/meta-rules-dat>
- ACL4SSR/ACL4SSR：<https://github.com/ACL4SSR/ACL4SSR>

采用前必须完成：

- 记录实际许可证与再分发义务；
- 固定 commit 或 Release；
- 保存源 URL、文件路径、SHA-256、抓取时间；
- 检查规则重叠、误杀与更新频率；
- 在发布包中附带必要的许可证和归属说明；
- 上游许可证不兼容或不明确时，只提供用户自行配置源 URL，不镜像再分发。

## 9. 参考其他脚本的边界

可以借鉴的场景：WARP、BBR、Swap、端口跳跃、诊断、清理、多客户端导出。

不得照搬：

- 未锁定版本的远程 `curl | bash`；
- 全局覆盖 nftables/iptables；
- 默认替换内核；
- 把密钥写入日志或公开订阅；
- 无状态迁移、无备份和无回滚的升级；
- 以协议数量作为主要质量指标。

## 10. 待补证据

以下项目需要用户提供固定版本或测试环境后再核验：

- Clash Verge Rev、Windows/Android v2rayN、具体 Android Clash 应用、Loon、Shadowrocket 的确切版本；
- Cloudflare 账户当前套餐和 Workers/KV 可用性；
- 目标域名、DNS 策略、云安全组限制；
- Debian 12、Ubuntu 24.04、arm64 与 1C1G/10GB 长时间数据；
- WARP 对目标 AI 服务的实际可用性与回退行为；
- 规则源最终选择与许可证处理方式。

---

# 11｜前期材料与环境准备

> 标题：VPSKit 前期材料与环境准备
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：在进入实现前明确凭据、域名、VPS、客户端、开发工具和产品决策

## 1. 本地凭据授权与使用边界

项目根目录的忽略文件 `.前期环境须知.md` 含用户为本任务主动提供的 root 密码和 Cloudflare API Token。该文件被 `.gitignore` 精确排除、当前未进入 Git 历史；用户已经明确授权 Agent 在 VPSKit、目标 VPS 和对应 Cloudflare 资源的任务范围内直接使用。Agent 在本机读取它不等于凭据已外泄，因此不要求仅因本次审查立即轮换。

真实操作时遵循：

1. 可以直接使用文件中的凭据连接目标 VPS，或调用授权范围内的 Cloudflare API；
2. 不在回复、工具摘要、日志、截图、方案文档或 Git 中显示完整值；
3. 不把凭据复制进源码、测试 fixture、ZIP、客户端配置或长期命令行参数；
4. 原生程序需要凭据时，优先使用标准输入、环境变量或权限为 `0600` 的短期凭据文件，避免出现在进程列表；
5. 只操作用户明确放入范围的 VPS、Zone、Worker、KV 和 DNS，不因 Token 权限较大而扩大任务；
6. 任务完成后清理本次产生的临时凭据副本，但不擅自删除用户原始文件；
7. 仅在发现真实泄露、异常登录、凭据进入 Git/公开渠道、用户要求，或短期凭据到期/任务结束时轮换；
8. 若凭据在执行中认证失败，先报告失效事实，不要求用户把新值粘贴到聊天，可由用户更新本地忽略文件后继续。

长期治理仍建议逐步采用 SSH 公钥和分角色 Token，但这属于改进方案，不是开始执行前的强制阻塞条件。

## 2. 本地开发环境

### 必需

| 项目         | 建议/要求                     | 用途             |
| ---------- | ------------------------- | -------------- |
| Windows    | 当前受支持 Windows 环境          | 本地开发与客户端验收     |
| PowerShell | PowerShell 7（`pwsh`）      | 构建、验收和安全脚本     |
| Git        | 当前稳定版                     | 分支、差异和版本管理     |
| Go         | 与 `go.mod` 一致，当前为 Go 1.26 | 编译、测试、生成 CLI   |
| SSH        | Windows OpenSSH Client    | 使用临时公钥连接测试 VPS |

当前 Codex 终端未在 `PATH` 中发现 Go，因此本次不能在本地重跑 `go test ./...`。进入代码实现前需要安装 Go 1.26，或明确以 GitHub Actions 作为构建测试环境并保存 CI 证据。

### 订阅模块需要

- Node.js 22 LTS 或实施时仍受 Wrangler 支持的 LTS 版本；
- Wrangler，固定版本并写入 lockfile；
- 独立的 Cloudflare 开发/预发布环境；
- 不含真实节点凭据的 fixture；
- 本地 Worker 测试和远端预发布测试脚本。

### 客户端验证需要

已确认的首批测试基线：

- 操作系统：Windows 11；
- Clash Verge Rev：`v2.5.2`；
- Mihomo 核心：`v1.19.29`；
- v2rayN Windows：`v7.23.1`。

仍待后续提供：

- v2rayN Android 的确切版本；
- Android “Clash” 的具体应用名、包名、版本和内置核心；
- iPhone/iPad 的 iOS 版本；
- Loon 和 Shadowrocket 的确切版本；
- 如继续支持 Hiddify，提供其版本与平台。

测试记录可使用脱敏截图和配置，必须删除 UUID、密码、私钥、订阅 Token、域名中不可公开的随机路径。

## 3. VPS 测试环境

### 第一阶段主环境

- 可随时重装的独立测试 VPS；
- Debian 13 amd64；
- 最低目标规格 1 vCPU、1GB RAM、10GB 磁盘；
- 一个可解析到 VPS 的测试域名；
- 明确云安全组和主机防火墙的 TCP/UDP 放行范围；
- 提供商控制台可用，可执行快照或重装；
- 时间同步正常，IPv4/IPv6 状态可确认。

### 后续矩阵

- Debian 12 amd64；
- Ubuntu 24.04 amd64；
- Debian 13 arm64；
- 至少一次 1C1G/10GB 长时间运行；
- 如实际存在 IPv6-only 或 NAT VPS，再作为独立 Profile 测试，不与普通公网 VPS 结论混用。

### 访问方式

用户只需准备：

1. 临时 Ed25519 公钥对应的登录授权；
2. 非交互 sudo 能力，或由用户在本地终端执行需要提权的命令；
3. VPS 提供商、地区、系统镜像、CPU 架构和资源规格；
4. 控制台重装/救援入口是否可用；
5. 当前防火墙与安全组的只读摘要。

不要在聊天中发送 root 密码、SSH 私钥、控制台 Cookie 或长期 API Token。

## 4. Cloudflare 材料与权限拆分

### 账户资源

- 一个处于 Active 状态的 Cloudflare Zone；
- 一个候选订阅子域，例如 `sub.example.com`，最终值由用户决定；
- Workers 与 KV 可用；
- 一个开发/预发布 Worker 和 KV namespace；
- 一个生产 Worker 和 KV namespace；
- 预算或套餐限制的确认结果。

2026-07-21 用户确认的准备状态：

- [x] 目标 Cloudflare Zone 为 Active；
- [x] `sub.<zone>` 和 `sub-dev.<zone>` 候选名称已确定；
- [x] 两个候选名称未发现冲突 DNS 记录；
- [x] Workers & Pages 控制台可访问；
- [x] Workers KV 控制台可访问；
- [x] 未提前创建生产 Worker、KV namespace 或订阅 DNS 记录。

具体主域名和凭据继续从用户授权的本地环境读取，不复制进公开方案包。

### 凭据角色

| 角色                  | 保存位置               | 最小用途                         | 是否可放 VPS      |
| ------------------- | ------------------ | ---------------------------- | ------------- |
| ACME DNS Token      | VPS 安全凭据文件         | 仅修改指定 Zone 的 DNS 记录用于证书      | 可，短期/最小权限     |
| Cloudflare 部署 Token | 本地可信环境或 CI Secret  | 部署 Worker、绑定 KV/域名           | 否             |
| KV 管理 Token         | 本地可信环境或 CI Secret  | 管理 namespace/初始化数据           | 否             |
| 节点发布凭据              | 单台 VPS 的 `0600` 文件 | 只允许该 `node_id` 发布规定 artifact | 可，必须可撤销       |
| 订阅读取 Token          | 客户端                | 读取单个订阅身份                     | 客户端保存，不复用发布凭据 |

Cloudflare 原生 KV 写权限是账户级，不能直接把它当作 node 级隔离。节点发布凭据的校验需要由发布 Worker/服务实现。正式权限模型未完成前，不要预先创建或长期保存高权限 Token。

## 5. 域名、端口和网络决策

用户需要准备或选择：

- Reality 使用的目标域名和允许的回退策略；
- Hysteria2 证书域名；
- 订阅域名；
- TCP/UDP 是否继续共用同一数字端口；
- 未来端口跳跃允许的 UDP 范围以及云安全组是否能同步放行；
- DNS 模式：系统 DNS、DoH/DoT、Fake-IP 或 redir-host；
- IPv6 是否启用、是否要求 AAAA；
- 客户端刷新间隔和规则刷新间隔；
- 订阅 Token 泄露时允许的过渡窗口。

所有端口和域名先写入 `plan`，由用户确认后再 apply。

## 6. 产品偏好确认表

实现前需要用户给出以下选择。若暂未决定，采用括号内建议默认值：

| 主题      | 需要选择                                 | 建议默认值                            |
| ------- | ------------------------------------ | -------------------------------- |
| 节点命名    | 提供商/国家/城市/协议格式                       | `{provider}-{region}-{protocol}` |
| 首个订阅客户端 | Mihomo、v2rayN、Loon、Shadowrocket      | Mihomo                           |
| 规则包     | minimal、cn-direct、antiad、ai-enhanced | `cn-direct-antiad`，标准强度          |
| 默认兜底    | DIRECT 或 PROXY                       | PROXY；局域网/中国规则优先直连               |
| DNS     | Fake-IP/非 Fake-IP、DoH 提供商            | 先按现有客户端习惯确定                      |
| 去广告     | 关闭/标准/严格                             | 标准，提供白名单                         |
| WARP    | 不启用/AI-only/custom/global            | 不默认启用；后续 AI-only                 |
| WARP 故障 | fail-open 或 fail-closed              | 可用性优先场景选 fail-open               |
| 快照加密    | age 收件人或口令                           | 优先 age 收件人；口令为备用                 |
| 快照恢复    | 保留或重生成协议凭据                           | 每次恢复时显式选择                        |
| 保留策略    | 本地备份、快照、发布修订数量                       | 先采用 3/3/2，再按空间观测调整               |
| 多 VPS   | 何时聚合、同名冲突策略                          | 单 VPS MVP 通过后再启用                 |

## 7. 规则数据准备

每个候选规则源准备一条记录：

```yaml
name: example
repository: https://github.com/owner/repo
revision: <commit-or-release>
path: rules/example.yaml
license: <spdx-id-or-review-required>
sha256: <digest>
behavior: domain
format: yaml
refresh_policy: build-time
```

另需准备：

- 必须直连的域名；
- 必须代理的域名；
- AI 服务域名偏好；
- 广告误杀白名单；
- 不允许上传或记录的隐私域名；
- 规则失效时的回退策略。

## 8. 分阶段就绪清单

### R0：可开始本地架构与 Renderer

- [ ] Go 1.26 可用或 CI 构建路径明确
- [x] 固定 Windows 11、Clash Verge Rev `v2.5.2`、Mihomo `v1.19.29` 和 v2rayN `v7.23.1` 测试基线
- [ ] 节点命名格式确认
- [ ] 无秘密 fixture 准备完成
- [ ] 当前 v0.1.0 回归基线保存

### R1：可开始预发布订阅

- [x] 本地凭据文件仍被 `.gitignore` 排除，且授权目标和用途已确认
- [x] Active Zone、Workers & Pages 和 Workers KV 控制台访问已确认
- [x] `sub.<zone>`、`sub-dev.<zone>` 候选订阅域名及无冲突状态已确认
- [x] 实现阶段已创建开发与正式 Worker/KV namespace，Custom Domain 健康检查通过
- [x] 部署管理 Token、节点发布 Secret、客户端读取 Token 已实际分离
- [x] 单 VPS 发布、回读、Token 轮换与吊销流程通过，日志和验收输出未包含完整凭据

### R2：可开始实机与恢复验收

- [ ] 可重装 Debian 13 测试 VPS 就绪
- [ ] 临时 SSH 公钥和撤销时间确认
- [ ] 快照/控制台救援入口可用
- [x] Windows 11 客户端版本清单完成
- [ ] iOS/Android 客户端版本清单完成
- [ ] 快照加密方式、恢复策略和保留数量确认
- [ ] 用户知晓端口、防火墙、WARP 和系统修改的影响范围

完成 R0 前不要开始大规模代码改造；R1 可使用用户已授权的本地管理凭据部署，但不把 Cloudflare 管理凭据常驻到 VPS；完成 R2 前不要把未验证客户端或恢复流程标为 stable。

---

# 12｜审查记录与修订说明

> 标题：VPSKit 功能演进方案审查记录与修订说明
>
> 生成时间：2026-07-21 16:35
>
> 生成者：Codex
>
> 版本：v0.3-R16
>
> 用途：记录已完成基线、用户筛选结果和暂停范围

## 1. 本轮输入

本轮以已完成的 v0.2.0 单 VPS 自动订阅部署与 Windows 11 客户端验收为基线。用户明确要求因实现额度有限，收敛原方案；并提供 `Android-ClashMeta-3xui-Google-All-Proxy-Dual-Schemes-v5.2-20260719.md` 作为 ACL4SSR 与 anti-AD 的方案 A/B 参考。

## 2. 已完成基线

- 单 VPS Cloudflare Workers/KV 自动订阅与生产发布；
- Mihomo/Clash Verge 和 v2rayN 基础订阅交付；
- 订阅读取令牌轮换、撤销、回读和完整修订发布；
- Windows 11 Clash Verge Rev 与 v2rayN 自动更新人工验收。

## 3. 保留的开发范围

1. 通用实例模型、Adapter、Renderer Registry；
2. 结构化 Mihomo Renderer；
3. 节点名称、地区、提供商、稳定 ID 等元数据；
4. ACL4SSR 分流、anti-AD、规则远程更新、fake-ip DNS、Sniffer、白名单和回滚；
5. `doctor --fix` 与 `system inspect`；
6. Salamander、拥塞控制/带宽建议、端口跳跃、UDP 调优；
7. Fail2ban、系统更新/重启需求、时间同步、DNS/IPv4/IPv6 健康检查。

## 3.1 本轮实施事实

- 方案 A 的 r0007 已通过 Windows 11 Clash Verge Rev 的加载、切换和实际连接验收；
- `v0.2.2-lab.2` 已将 20 个 ACL4SSR、MetaCubeX Google 与 anti-AD 来源下载、限额检查、SHA-256 记录并发布为 r0008 的受管规则工件；r0008 已通过 Windows 11 Clash Verge Rev 更新、切换和实际连接验收；
- `doctor --fix`、`system inspect`、Fail2ban SSH jail 已在 Debian 13 amd64 实机通过；
- Fail2ban 使用现有 `sshd` jail 的 VPSKit 覆盖文件，完整验证应用、删除和重新应用；
- `v0.2.3-lab.1` 已完成 schema 8→9、精确域名白名单与自定义规则的添加/删除、最终空状态和 r0012 全目标回读；测试只使用 `.invalid` 保留域名，不影响真实流量。
- 方案 B r0014 已通过 Windows 11 Clash Verge Rev 更新、切换和实际使用验收；真实误杀域名白名单命中、Hysteria2 性能基准、端口跳跃和 UDP 调优仍未完成。系统更新候选检查与 Fail2ban SSH 白名单均已完成；规则来源受管缓存已完成，但上游候选的许可证登记和格式 smoke test 仍待补充。

## 3.2 R14 实施事实

- `v0.2.4-lab.2` 的签名包与内置 `bundle verify` 通过，Debian 13 amd64 原位升级前已备份旧 CLI；
- Fail2ban 白名单命令只接受单个 IP/CIDR，写入 VPSKit 所有的 `sshd` 覆盖文件，最多保留 32 条规范化条目；
- 初次实机验证暴露出 Fail2ban systemd restart 成功后控制 socket 仍短暂未就绪的竞态。实现改为最多等待 15 秒确认 jail active；失败路径恢复原文件并重启服务，未残留测试条目；
- 最终已用保留测试地址完成“添加 → jail active → 删除 → 空状态”闭环，Fail2ban、Xray 与 sing-box 均保持 active；该变更不改订阅与客户端配置；
- 方案 B `acl4ssr` 的 r0014（19 个受管规则源，不含 anti-AD）已由用户确认更新、切换和实际使用通过。

## 3.3 R15 实施事实

- `v0.2.5-lab.1` 将规则刷新改为临时目录下载、manifest 完整写入后原子激活，失败时自动清除临时目录；
- 本地单元测试已验证上游 503 时活动 revision 不变、候选 revision 与临时目录不残留；完整下载时只激活完整 revision；
- Debian 13 使用只对刷新子进程生效的无效 HTTP(S) 代理模拟上游不可达，命令按预期失败；状态、订阅状态和活动 manifest 摘要不变，候选 revision 不存在，Xray/sing-box 均保持 active；
- 该故障保护不下发客户端修订，因此无需额外 Windows 客户端手工验收。

## 3.4 R16 实施事实

- 通过 Chrome 只读核验 sing-box、Hysteria2 和 Mihomo 官方文档及源码，确认当前 sing-box `1.13.14` / Mihomo `1.19.29` 的安全边界；
- Salamander 可进入独立实验切片；Gecko、`bbr_profile` 因服务端需 `>=1.14.0` 被阻止；端口跳跃因需要独立 NAT、云安全组与回滚组件被阻止；
- `v0.2.6-lab.1` 新增 `hysteria2 inspect`，在 Debian 13 读取到 UDP/443 监听、`rmem_max/wmem_max=212992`，并且执行前后状态和订阅摘要不变；
- 该只读切片不修改现有节点或客户端配置，Xray 与 sing-box 均保持 active。

## 3.5 R17 实施事实

- `v0.2.7-lab.1` 新增 Salamander `plan|enable|disable`，默认关闭，启用时为 Hysteria2 生成独立混淆密码并事务重渲染服务端及全部既有客户端交付产物；
- Debian 13 amd64 已完成签名包校验、只读预演、启用、r0015 自动发布、受管配置/UDP/服务/doctor 回读；
- 用户已确认 Windows 11 Clash Verge Rev/Mihomo 的既有订阅更新、Hysteria2 节点切换与实际使用均通过；
- Gecko、`bbr_profile` 与端口跳跃仍未开放，保留既有版本与网络前置条件。

## 4. 规则决策

- 方案 A 为默认：ACL4SSR + anti-AD；
- 方案 B 为兼容回退：仅 ACL4SSR；
- 两者都含 fake-ip DNS 和 Sniffer；
- VPSKit 刷新时下载并哈希 Provider，客户端通过受读取 Token 保护的 Worker 规则地址按 24 小时周期检查更新；
- Google/Gemini 专项规则优先于 UnBan 与 anti-AD；
- anti-AD 误杀优先通过精确白名单解决，频繁误杀时切换方案 B；
- 规则源 revision 已可追溯、校验并与订阅修订一起回滚；不把不受控上游分支直接当作稳定发布承诺。

## 5. 暂停范围

加密快照、少量多 VPS、Loon/Shadowrocket、WARP、AnyTLS、XHTTP + REALITY、TUIC、通用 BBR、Swap、通用防火墙管理、Docker、Web 面板、多租户和计费均不进入当前路线。

## 6. 文档同步范围

本次已同步执行摘要、规则方案、Hysteria2/系统模块、路线图、README、完整版和文件清单。ZIP 仅作为本地可下载副本，不纳入 Git。
