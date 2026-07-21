# VPSKit 功能演进完整方案 v0.3-R1

> 标题：VPSKit 功能演进完整方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：VPSKit 后续版本架构、订阅、规则、恢复、可选功能、验收和前期准备的统一实施依据

- 原编制日期：2026-07-20
- 审查修订日期：2026-07-21
- 源码审查基线：`main@22b6457db557b3fc3e4b723784e33d6d55f563fe`
- 当前产品基线：VPSKit v0.1.0
- 证据原则：规划、实现、解析、实机和长期运行分级表述；未验证能力不得标为 stable

本文件由同目录 00–12 分卷按顺序机械合并。出现歧义时，以分卷、`FILE-MANIFEST.md` 和对应源码审查基线共同判定。

## 目录

1. 00｜执行摘要与决策清单
2. 01｜现状审计与产品边界
3. 02｜目标架构与扩展模型
4. 03｜自动订阅与多客户端交付
5. 04｜分流规则与去广告方案
6. 05｜WARP 与 AI 出站方案
7. 06｜恢复、多 VPS 与运维闭环
8. 07｜系统工具与高级功能
9. 08｜版本路线图与优先级
10. 09｜测试验收与发布门禁
11. 10｜参考项目与资料
12. 11｜前期材料与环境准备
13. 12｜审查记录与修订说明

---

# 00｜执行摘要与决策清单

> 标题：VPSKit 功能演进执行摘要与决策清单
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：给出源码审查后的产品结论、关键修订和执行优先级

## 1. 审核结论

结论：**产品方向通过，但原路线图和订阅安全模型需要先修订再开发。**

当前 `v0.1.0` 已经完成质量较高的部署核心。经本地源码、公开仓库和实机验收文档对照，以下能力属于已确认基线：

- 固定 Release 安装、签名清单、摘要、SBOM 和 attestation；
- Xray 承载 VLESS + REALITY + Vision；
- sing-box 承载 Hysteria2 + TLS；
- TCP/UDP 共用同一数字端口；
- 配置、状态、备份、导出和变更修订的一致性；
- 事务化修改、失败回滚、受管卸载和孤儿扫描；
- Mihomo、sing-box、分享链接、二维码和安全客户端 ZIP；
- Debian 13 amd64 普通用户端到端实机验收。

后续不应重写这个闭环，而应通过小步垂直切片逐步扩展。

## 2. 本次必须修正的六项决策

### 2.1 架构不能等到 v0.7

当前状态仍固定绑定 `Reality`、`Hysteria2` 和两个核心，`internal/app/app.go` 约 1484 行，Mihomo 仍由字符串拼接生成。若先堆自动订阅、多客户端和恢复，再到 v0.7 才抽象接口，会把新逻辑继续压入现有集中层。

修订为：

- v0.1.1 先建立节点元数据、结构化 Mihomo 模型、Renderer/Publisher 最小接口；
- 后续每个版本沿接口增加一个可验收的垂直切片；
- 完整 `instances[]` 迁移必须在第三协议之前完成，但不做一次性大重构。

### 2.2 自动订阅先做单 VPS 最小闭环

首个订阅版本只承诺：

1. 单 VPS；
2. Mihomo 完整配置；
3. v2rayN 节点订阅；
4. 静态导出兜底；
5. 读取 Token 轮换；
6. 发布失败不影响现有节点。

Loon、Shadowrocket、多 VPS 聚合和规则远程更新在这个闭环稳定后加入。

### 2.3 不向 VPS 下发 Cloudflare 管理级 KV Token

Cloudflare 的 Workers/KV 写权限属于账户或区域资源权限，不能据此实现“每台 VPS 只能写自己的某个 KV key”。因此原方案“每台 VPS 独立 API Token，单台泄露不能修改其他节点”的表述证据不足。

修订为：

```text
VPS ──节点级发布凭据──> Worker 受认证发布入口
Worker ──KV Binding──> 写入该 node_id 的命名空间
客户端 ──读取 Token──> Worker 只读交付入口
```

Cloudflare 管理级 Token 只用于本地或受保护 CI 部署 Worker，不常驻 VPS。

### 2.4 KV 发布不是全局原子切换

Workers KV 是最终一致性存储，某些地区可能在缓存 TTL 内继续读取旧值。发布语义只能承诺：

- 每个已发布对象本身完整；
- 客户端可能暂时读到旧的完整修订或新的完整修订；
- 不宣称所有地区同一时刻切换；
- 发布完成需经过回读与收敛观察；
- 失败时可重新激活上一完整修订。

### 2.5 Hysteria2 新字段必须绑定核心版本

当前产品锁定 `sing-box v1.13.14`。Gecko、`bbr_profile`、随机跳跃上限和 Realm 等能力在上游文档中属于 `1.14.0` 变化，不能直接列为当前核心可用功能。

修订为：先完成独立核心升级门和客户端矩阵，再开放相应菜单；Salamander 与端口跳跃也必须按服务端防火墙实现方式单独验收。

### 2.6 本地凭据作为授权操作入口

现有本机忽略文件中的 root 密码和 Cloudflare Token 是用户为受控执行明确提供的凭据。文件被 `.gitignore` 排除且没有进入 Git；Agent 在本地读取并不等于凭据已经外泄，也不构成强制轮换条件。

修订后的边界是：允许 Agent 在用户授权的 VPSKit/VPS/Cloudflare 任务范围内直接使用这些凭据，但不得在回复、日志、方案包或 Git 中回显；不得用于无关账户或扩大权限。只有发现真实泄露证据、用户要求、权限用途发生变化，或凭据本来就是短期凭据且任务结束时，才执行轮换或撤销。

## 3. 用户需求结论

当前明确需求：

- 首批测试平台：Windows 11；
- Windows 固定测试基线：Clash Verge Rev `v2.5.2`，Mihomo 核心 `v1.19.29`，v2rayN `v7.23.1`；
- iOS：Loon、Shadowrocket；
- Android：Clash 系客户端、v2rayN、Hiddify；
- 希望使用固定、可自动更新的订阅地址；
- 希望节点、策略组、分流规则和去广告规则共同更新；
- 有 OpenAI、Gemini、Claude 等 AI 服务访问需求；
- 当前未出现明显 UDP 封锁；
- 系统功能可后置，但最终希望加入；
- VPS 数量少，未来预计约 2～10 台；
- 重装后既要支持全新凭据，也要支持恢复；
- 偏好功能丰富，但不需要多租户、计费和机场管理。

## 4. 修订后的优先级

### P0：立即进入下一版本

1. 自定义节点名称、地区、提供商和稳定节点 ID；
2. Mihomo 结构化 YAML 渲染与固定版本解析测试；
3. Renderer capability/compatibility profile；
4. 本地订阅产物模型、修订清单和摘要；
5. 静态发布后端，先验证生成与回滚；
6. 单 VPS Workers 订阅最小闭环；
7. 节点级发布入口与 Cloudflare 管理凭据分离；
8. cleanup plan/apply；
9. allowlist 驱动的脱敏诊断包；
10. 当前敏感准备材料轮换与迁移。

### P1：P0 稳定后

1. 分流、DNS 和标准去广告规则包；
2. v2rayN 路由规则独立交付；
3. 可移植加密快照；
4. Loon Renderer 实机验证；
5. Shadowrocket Renderer 实机验证；
6. 多 VPS 聚合和故障节点禁用；
7. `doctor --fix` 安全修复子集；
8. 1C1G/10G 长期资源观察。

### P2：按明确需求和证据加入

1. Hysteria2 Salamander；
2. Hysteria2 端口跳跃；
3. sing-box 1.14+ 升级后的 Gecko/BBR profile/Realm；
4. WARP AI-only 精确出站；
5. AnyTLS；
6. XHTTP + REALITY；
7. TUIC；
8. UFW、BBR、Swap、Fail2ban 和内置 HTTPS 订阅后端。

## 5. 明确不进入核心

- 多用户、多租户、配额和流量计费；
- 机场面板、注册、套餐和邀请码；
- 默认安装 Docker、数据库或 Web 面板；
- DD 重装、BBR Plus、锐速和第三方魔改内核；
- 默认部署大量协议；
- 默认将全部流量送入 WARP；
- HTTPS MITM、用户 CA、脚本改写式去广告；
- 把客户端 User-Agent 自动识别作为唯一订阅入口；
- 把“AI 服务可访问”写成长期保证。

## 6. 推荐开发顺序

```text
Renderer/元数据基础
→ 单 VPS 自动订阅 MVP
→ 分流、DNS 与去广告
→ 加密可移植快照
→ 更多客户端与多 VPS
→ Hysteria2 增强
→ WARP AI 出站
→ 通用实例迁移完成后新增协议
→ 系统工具
→ v1.0 收口
```

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

| 功能               | 定位      | 默认状态         |
| ---------------- | ------- | ------------ |
| Reality/Hy2 生命周期 | 产品核心    | 按 Profile 启用 |
| Renderer 和静态导出   | 产品核心    | 启用           |
| 自动订阅 Publisher   | 客户端交付核心 | 用户配置后启用      |
| 规则与去广告           | 客户端交付模块 | 标准规则可选       |
| 本地备份与恢复          | 产品核心    | 启用           |
| 可移植加密快照          | 生命周期扩展  | 显式导出         |
| 多 VPS 聚合         | 少量节点协同  | 显式启用         |
| WARP             | 可选出站模块  | 默认不安装        |
| UFW/BBR/Swap     | 可选系统模块  | 默认不修改        |
| Web 面板/数据库       | 暂不加入    | 不适用          |
| Docker Backend   | 实验/后置   | 默认不启用        |
| DD/魔改内核          | 不加入     | 不适用          |

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

| 凭据                  | 用途                         | 存放位置                   | 泄露影响                   |
| ------------------- | -------------------------- | ---------------------- | ---------------------- |
| Cloudflare 管理 Token | 部署 Worker/KV/Custom Domain | 本地凭据库或受保护 CI           | 可修改 Cloudflare 资源，最高风险 |
| 节点级发布凭据             | 某台 VPS 发布自己的 node_id       | VPS `0600` secret file | 只能替换该 node_id 的候选内容    |
| 订阅读取 Token          | 客户端 GET 订阅                 | 各客户端                   | 可读取节点凭据，不能发布           |

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

> 标题：VPSKit 分流规则与去广告方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：定义规则包、DNS 配套、来源治理、客户端适配和误杀回滚边界

## 1. 设计原则

分流和去广告属于客户端交付层，不属于服务端入站协议层。

因此：

- 不写入 Xray/Hy2 入站；
- 不改变 Reality/Hysteria2 凭据；
- 切换规则包只更新客户端产物；
- 规则失败不重启服务端；
- 规则可独立更新和回滚；
- DNS 策略必须与规则一起设计；
- 不把第三方规则仓库名称直接当作 VPSKit 的稳定产品契约。

## 2. 规则包

### 2.1 `minimal`

```text
LAN / 私网 → DIRECT
其他 → PROXY
```

最少依赖，适合排障和订阅 MVP。

### 2.2 `cn-direct`

```text
用户白名单 → DIRECT
LAN / 私网 → DIRECT
中国大陆常用域名/IP → DIRECT
其他 → PROXY
```

作为日常分流的默认候选。是否启用取决于用户实际网络位置和 DNS 策略。

### 2.3 `cn-direct-antiad`

在 `cn-direct` 上增加高置信广告、跟踪和恶意域名阻断。

### 2.4 `ai-enhanced`

```text
OpenAI / ChatGPT / Codex → AI
Gemini / Google AI → AI
Claude / Anthropic → AI
GitHub / Copilot → PROXY
中国大陆服务 → DIRECT
高置信广告 → REJECT
其他 → PROXY
```

未启用 WARP 时，`AI` 指向普通代理策略；启用 WARP 后，`AI` 可选 WARP 出站。

### 2.5 第三方兼容 Profile

可以提供：

- `source-acl4ssr`；
- `source-anti-ad`；
- `source-meta-rules`。

这些是**规则来源 Profile**，不是 VPSKit 自有规则语义。启用时必须记录 commit/tag、摘要、许可证、转换方式和归属说明。

## 3. 去广告等级

### 关闭

仅分流，不阻断广告。

### 标准

只使用高置信域名，默认推荐。

### 严格

加入更多跟踪、遥测和可疑域名，可能造成登录、支付、验证码、图片或 App 启动异常，必须显式选择。

首版不做按客户端自动猜测等级。

## 4. 规则优先级

建议：

```text
用户白名单
→ 用户自定义 DIRECT/PROXY/AI/REJECT
→ LAN 与系统必要域名
→ AI 专用规则
→ 高置信广告/恶意域名
→ 应用与服务分流
→ 国内外基础分流
→ FINAL
```

白名单必须早于广告规则。用户自定义规则变更必须经过 lint 和冲突报告。

## 5. DNS 配套

没有 DNS 设计的分流配置可能出现：

- 代理域名被本地污染；
- DNS 查询绕过预期策略；
- IP 规则触发错误解析；
- rule-provider 域名无法更新；
- 启动时产生 DNS/bootstrap 循环。

Mihomo 完整配置至少定义：

- bootstrap DNS；
- 直连 DNS；
- 代理/远程 DNS；
- 节点域名解析路径；
- rule-provider 下载路径；
- IPv6 启用策略；
- DNS 失败时的回退行为。

DNS 具体字段必须绑定固定 Mihomo 版本，不能仅复制社区模板。

## 6. 白名单和自定义规则

逻辑集合：

```text
custom_whitelist
custom_direct
custom_proxy
custom_ai
custom_reject
```

要求：

- 可导出、导入和备份；
- 每条规则记录来源：用户/内置/第三方；
- 重复、遮蔽和永不命中规则给出警告；
- 自定义白名单只影响客户端，不触发服务端重启；
- 用户规则中不得出现订阅读取 Token 或协议私钥。

## 7. 规则来源治理

每个来源记录：

- 名称和用途；
- 官方仓库/上游 URL；
- tag、commit 或固定 Release；
- 下载摘要；
- 原格式和目标格式；
- behavior；
- 许可证与 attribution；
- 允许的自动更新策略；
- 最近成功修订和上一可回滚修订；
- 上游删除或许可证变化时的处置。

禁止：

- 客户端每次刷新时直接拉不固定的 `main`；
- 无摘要、无来源、无许可证检查的规则进入 stable；
- 将 `domain`、`ipcidr`、`classical` 混用；
- 将规则源更新与节点凭据更新绑定为同一不可回滚动作；
- 在公共规则资源中加入用户节点或订阅凭据。

## 8. 公共规则与私密订阅分离

广告、AI 分类、常规分流等规则本身不含节点秘密，建议通过公共、不可变、版本化路径提供：

```text
https://sub.example.com/rules/<pack>/<revision>/ads.mrs
https://sub.example.com/rules/<pack>/<revision>/ai.mrs
```

主订阅仍受 read-token 保护。这样远程规则 URL 无需复制订阅 Token，降低日志和分享时的泄露面。

用户私有规则可：

- 小规模时内联到受保护主配置；
- 规模较大时使用单独私密资源和独立 Token；
- 不复用节点订阅读取 Token。

## 9. Mihomo 实现

示意：

```yaml
rule-providers:
  ads:
    type: http
    behavior: domain
    format: mrs
    url: https://sub.example.com/rules/cn-direct-antiad/r0007/ads.mrs
    path: ./rules/vpskit-ads-r0007.mrs
    interval: 86400

rules:
  - RULE-SET,ads,REJECT
  - RULE-SET,ai,AI
  - RULE-SET,google,PROXY
  - GEOIP,CN,DIRECT
  - MATCH,PROXY
```

注意：

- `mrs` 只适用于 Mihomo 支持的 domain/ipcidr behavior；
- `classical` 规则使用对应 YAML/text 格式；
- `path` 必须唯一且位于客户端允许目录；
- 必须给 rule-provider 设置合理大小上限；
- 远程更新失败时保留本地缓存。

## 10. v2rayN 实现

- 节点订阅不混入路由规则；
- 路由通过独立 URL 导入；
- Xray 和 sing-box 核心可能需要不同规则产物；
- 用户需要首次选择/导入路由；
- stable 前验证 Windows/Android UI 的 URL 更新行为；
- 不宣称节点订阅会自动覆盖用户现有路由设置。

## 11. Loon/Shadowrocket 实现

- Loon 按官方远端配置/节点/规则入口生成；
- Shadowrocket 以真实 App 版本和可回滚实机样本为准；
- 两者的规则语法、策略名称、更新按钮行为分别测试；
- 不用一份文本模板假设两者完全兼容；
- 只有导入、刷新、命中、误杀白名单和 Token 轮换均通过后才标 stable。

## 12. 去广告能力边界

域名/IP 规则可处理：

- 第三方广告域名；
- 跟踪和统计域名；
- 部分启动广告；
- 部分恶意域名。

不能保证处理：

- YouTube 等内容流内嵌广告；
- 广告和正常内容共域；
- 服务端直接拼接内容；
- 必须 HTTPS 解密才能识别的请求。

首版禁止：

- MITM；
- 安装用户 CA；
- HTTPS 解密；
- 重写脚本；
- 修改 App 返回内容。

## 13. 更新与回滚

```text
下载固定来源
→ 校验摘要、大小、许可证元数据和格式
→ 转换目标格式
→ lint 与冲突检查
→ 固定版本客户端解析
→ 规则 smoke test
→ 发布新 ruleset_revision
→ 回读
→ 保留上一版本
```

异常时：

- 不修改节点和服务端；
- 不发布空规则或截断文件；
- 重新激活上一完整规则修订；
- 客户端继续使用旧缓存；
- `doctor` 显示来源、修订和失败原因，但不打印私密 URL。

---

# 05｜WARP 与 AI 出站方案

> 标题：VPSKit WARP 与 AI 出站方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：说明 WARP 的真实作用、可选实现、隐私回退、资源门槛和验收边界

## 1. WARP 的作用

WARP 改变的是 VPS 到目标网站的出口路径：

```text
客户端 → Reality/Hy2 → VPS 原生出口 → 目标服务
```

启用后可变为：

```text
客户端 → Reality/Hy2 → VPS → 本地 WARP 代理 → Cloudflare 网络 → 目标服务
```

它不直接优化客户端到 VPS 的链路，所以不能据此解释或修复 Reality 比 Hy2 慢。

## 2. 可能有帮助的场景

- VPS 原生 IP 被某些服务限制；
- 原生 IP 信誉较差；
- IPv4/IPv6 出口或路由异常；
- VPS 到特定服务的原生路径较差；
- 希望 AI 域名与普通流量使用不同出口。

## 3. 不能承诺

- 不保证解锁 OpenAI、Gemini 或 Claude；
- 不保证固定国家或地区；
- 不保证出口 IP 长期稳定；
- 不保证延迟或吞吐改善；
- 不保证 Cloudflare 出口不会被目标服务限制；
- 不替代优质 VPS 线路；
- 不规避目标服务的账号、地区或使用条款。

文档和菜单只能表述为“出口切换与可达性 A/B 测试”，不能写“AI 解锁”。

## 4. 推荐实现

优先使用 Cloudflare 官方 Linux WARP 客户端的本地代理能力，不采用来源不明的 WARP 注册脚本或凭据。

```text
sing-box outbound
├── direct
└── warp-socks/http → WARP Local Proxy

route
├── AI 规则 → warp
└── 其他 → direct
```

Cloudflare 的 WARP 模式和命令可能随客户端版本变化。实现时必须：

1. 固定 `cloudflare-warp` 包版本和仓库来源；
2. 运行目标版本的 `warp-cli --help` / `warp-cli mode --help`；
3. 探测 Local Proxy 是否在该 Linux 版本可用；
4. 记录监听地址、端口和模式；
5. 不在方案中硬编码未经目标版本验证的子命令。

## 5. 模式

### `ai-only`

仅 AI 服务走 WARP，推荐默认模式。

### `custom`

用户指定域名/规则集走 WARP。

### `global`

全部代理出口走 WARP，高级模式，默认关闭。它更容易增加延迟、改变地区并影响非 AI 服务。

## 6. 故障回退和隐私选择

启用时要求用户明确选择：

### availability-first

```text
AI → WARP
WARP 不可用 → direct
```

优点是可用性高；风险是 WARP 故障时会暴露 VPS 原生出口。

### privacy-first

```text
AI → WARP
WARP 不可用 → block
```

适合不希望 AI 流量回落到原生 IP 的场景。

不能在未提示用户的情况下自动从 `block` 改为 `direct`。

## 7. 命令建议

```bash
vpskit feature warp preflight
vpskit feature warp plan
vpskit feature warp install
vpskit feature warp status
vpskit feature warp test --compare-direct
vpskit feature warp route-pack ai
vpskit feature warp route custom
vpskit feature warp disable
vpskit feature warp rollback
vpskit feature warp remove
```

## 8. 健康检查

检查：

- WARP 包版本和服务状态；
- Local Proxy 是否仅监听 loopback；
- direct 与 WARP 出口 IPv4/IPv6/ASN；
- direct 与 WARP 的 TLS、HTTP、延迟和基础吞吐对比；
- OpenAI/Gemini/Claude 等目标的匿名可达性；
- 失败回退是否符合用户选择；
- SSH、Reality 和 Hy2 入站是否不受影响。

“首页返回 200”不能证明登录后完整 AI 功能可用。需要用户以自己的合法账号做最终人工验证，且不得把 Cookie、会话或账号 Token交给 VPSKit。

## 9. 常驻资源

WARP 需要常驻进程。资源策略：

- 默认不安装；
- 安装前记录可用内存、swap、磁盘和当前核心 RSS；
- 启用后记录 idle RSS、CPU 和句柄/连接数；
- 1C1G 上分别测试空闲、更新解压、订阅生成和传输压力；
- 不先写死官方“最低内存”作为本项目资源承诺；
- 若可用内存或磁盘低于项目安全阈值，阻止安装并给出计划结果。

## 10. 安全边界

- Local Proxy 只监听 `127.0.0.1`/`::1`；
- 不接管 SSH；
- 不修改系统默认路由；
- 不把所有系统流量默认送入 WARP；
- 不把 WARP 注册信息写入客户端订阅或诊断包；
- WARP 日志不得记录目标完整 URL 或订阅 Token；
- 可以独立禁用和卸载；
- 卸载后恢复 direct 或 block 的明确状态；
- 失败不影响 Xray/Hy2 入站。

## 11. AI 规则包

首批候选：

- OpenAI / ChatGPT / Codex；
- Anthropic / Claude；
- Gemini / Google AI Studio / Generative Language API；
- GitHub Copilot。

规则必须可审计和覆盖。不要把所有 Google、Microsoft 或 GitHub 流量无条件送入 WARP。

## 12. 发布门槛

WARP 功能标 stable 前必须具备：

- Debian 13 amd64 安装、禁用、卸载实机；
- 固定官方包版本；
- loopback 监听证明；
- availability-first 与 privacy-first 两种故障测试；
- 1C1G 资源变化；
- direct/WARP A/B 报告；
- 至少一次系统重启持久化；
- 不影响 SSH、Xray、sing-box 和证书续期；
- 用户实际 AI 服务验证记录，但不包含账号秘密。

---

# 06｜恢复、多 VPS 与运维闭环

> 标题：VPSKit 恢复、多 VPS 与运维闭环方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：区分本地备份与可移植快照，定义重装恢复、多 VPS、修复、清理和诊断边界

## 1. 先区分两类恢复能力

### 1.1 当前本地 backup/restore

v0.1.0 已有受管备份与恢复，目标是同一台 VPS 上的事务回滚和配置恢复。它不是面向长期离线保存的加密灾备包。

### 1.2 未来 portable snapshot

可移植快照用于：

- VPS 重装；
- 更换系统或架构；
- 离线保存；
- 恢复节点身份、规则和订阅关系。

文档、命令和状态必须使用不同名称，避免用户把本地备份误认为已加密的离线灾备。

## 2. 两种重装模式

### 全新部署

- 生成新 UUID、Reality 密钥和 Hy2 密码；
- 重新签发证书；
- 生成新节点/订阅身份或替换旧节点；
- 吊销旧订阅 Token；
- 适合凭据可能泄露或希望完全重置。

### 恢复部署

- 恢复节点元数据和配置偏好；
- 按策略恢复或轮换协议凭据；
- 恢复规则选择；
- 重建订阅关系；
- 重新下载当前架构的固定核心二进制；
- 重新签发或校验证书；
- WARP 默认重新注册，不直接搬运设备注册状态。

## 3. 可移植加密快照

命令：

```bash
vpskit snapshot export
vpskit snapshot inspect <file>
vpskit snapshot restore <file> --plan
vpskit snapshot restore <file> --apply
```

建议结构：

```text
snapshot.age
├── encrypted payload
│   ├── state.json
│   ├── secrets/
│   ├── certificates/
│   ├── versions.lock
│   ├── ownership.json
│   ├── subscription.json
│   ├── ruleset.json
│   └── manifest.json
└── minimal public envelope
    ├── snapshot_schema
    ├── created_at
    ├── encryption_type
    └── encrypted_sha256
```

公共 envelope 不记录 IP、域名、node ID、客户端名称或服务商。

## 4. 加密与口令处理

优先顺序：

1. 用户提供的 age 公钥；
2. age 口令模式；
3. 未来可选硬件/密钥库集成。

要求：

- 固定 age 实现和版本，纳入签名资产与许可证清单；
- 口令通过 TTY 或文件描述符输入，不进入 argv、环境变量、日志或 shell history；
- 导出后立即执行完整性读回；
- 错误口令和篡改必须在任何写入前失败；
- 不支持普通 ZIP 密码或明文 tar；
- 快照文件和解密口令不得放在同一位置。

## 5. 快照内容边界

包含：

- 状态、节点元数据和实例配置；
- 协议 secrets；
- 受管证书和私钥；
- 版本锁、ownership、schema；
- 订阅 endpoint 元数据、read-token 策略和 node publisher 身份；
- 规则选择和用户自定义规则；
- 最近有效 Artifact Set 的摘要和可选加密副本；
- 恢复说明。

默认不包含：

- SSH 私钥；
- Cloudflare 管理级 Token；
- root 密码；
- 客户端 Cookie/账号会话；
- WARP 设备注册材料；
- 缓存、普通日志和可重新下载的核心二进制。

Cloudflare ACME Token 是否包含必须由用户显式选择；默认建议恢复时重新注入或轮换。

## 6. 恢复策略

```text
restore-identical
restore-and-rotate-subscription
restore-and-regenerate-protocol-secrets
restore-config-only
```

### identical

恢复协议凭据、节点身份和订阅读取地址。若旧 VPS 可能被攻破，不得选择。

### rotate-subscription

协议凭据不变，轮换读取和发布凭据。

### regenerate-protocol-secrets

恢复结构、规则和节点名，但重新生成协议凭据并要求客户端更新。

### config-only

只恢复非敏感配置偏好，不恢复密钥、证书或 Token。

## 7. 恢复预演

`--plan` 必须零写入并输出：

- snapshot/schema/VPSKit 兼容性；
- 系统、架构和磁盘条件；
- 将创建、覆盖或跳过的受管对象；
- 端口和服务冲突；
- 域名、DNS 和证书要求；
- 需要重新下载的当前架构二进制；
- 需要重新签发的证书；
- 是否保留订阅地址；
- 将轮换的凭据；
- 客户端是否必须更新；
- 上游固定版本是否仍可获取；
- 回滚点和失败策略。

跨 amd64/arm64 恢复只恢复可移植状态，绝不复用旧架构二进制。

## 8. 少量多 VPS

目标规模约 2～10 台，不建设远程 shell 控制面。

每台 VPS：

- 独立 node ID、状态和协议凭据；
- 独立节点级发布凭据；
- 独立本地备份和加密快照；
- 独立健康状态；
- 只发布自己的 Node Manifest。

订阅聚合器：

- 聚合节点和策略组；
- 标记地区、提供商和优先级；
- 保留上一有效节点修订；
- 可禁用故障节点；
- 不保存 SSH 密码；
- 不持有 root 权限；
- 不远程执行系统命令。

## 9. 节点命名与冲突处理

显示名建议：

```text
<提供商>-<国家/城市>-<序号>-<协议>
Provider-JP-Tokyo-01-Reality
Provider-JP-Tokyo-01-Hysteria2
```

稳定 identity 使用 `node_id`，显示名可变。聚合时若重名：

1. 添加用户可读序号；
2. 仍冲突时添加 node ID 短后缀；
3. 不用 IP、UUID 或 Token 解决冲突。

## 10. 聚合策略

```text
PERFORMANCE：Hy2 优先，基于用户选择，不凭单次延迟自动判定
STABLE：Reality 优先
REGION-*：按地区分组
AI：具备所选 AI 出站策略的节点
MANUAL：全部可用节点手选
```

健康信息只用于提示和可选禁用，不在首版做自动频繁切换。

## 11. doctor --fix

允许自动修复：

- 受管文件权限；
- 缺失受管目录；
- systemd daemon-reload；
- 配置有效但未启用的 VPSKit 服务；
- 可重建缓存；
- 未完成事务恢复；
- 订阅元数据与本地产物摘要不一致。

禁止自动：

- 修改 SSH；
- 清空或重写未知防火墙；
- 替换内核；
- 关闭未知服务；
- 删除唯一备份；
- 更新系统全部软件包；
- 轮换协议/订阅凭据；
- 重新签发证书；
- 将 WARP 隐私回退从 block 改为 direct。

每项 fix 必须有 plan、执行记录和反向操作。

## 12. cleanup

```bash
vpskit cleanup plan
vpskit cleanup apply --plan-id <id>
```

候选范围：

- 已结束旧事务；
- 失败 staging；
- 更新缓存；
- 过期客户端 ZIP；
- 旧规则缓存；
- 超额日志；
- 多余核心版本；
- 超过保留数量的备份和发布修订。

必须保留：

- 当前和上一成功核心版本；
- 当前状态和最近一次成功迁移前状态；
- 至少一个可恢复本地备份；
- 当前和上一客户端/规则发布；
- 尚未确认下载的最新快照；
- ownership 和审计索引。

`plan-id` 应绑定候选清单摘要，防止计划显示后目录变化导致误删。

## 13. 脱敏诊断包

```bash
vpskit support bundle
```

采用**字段 allowlist**，不是“收集所有文件后做字符串替换”。允许内容：

- 系统、内核、架构；
- 内存、磁盘、inode；
- 服务状态和受管端口；
- 核心版本；
- 配置/规则/发布摘要；
- 脱敏的结构化错误码；
- 最近事务状态。

不得包含：

- 协议 UUID/密码/私钥；
- TLS 私钥；
- Cloudflare/WARP/发布凭据；
- 完整订阅 URL；
- SSH 密钥或密码；
- 真实客户端配置；
- shell history、环境变量全集或任意目录 dump。

生成后运行二次 secret scan；发现疑似秘密时失败关闭，不生成“可能已脱敏”的包。

---

# 07｜系统工具与高级功能

> 标题：VPSKit 系统工具与高级功能方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：定义系统模块、Hysteria2 版本门、新协议候选和明确排除项

## 1. 原则

系统功能最终可以加入，但不能成为 Reality/Hy2 安装前置。

所有模块必须：

- 独立命令和状态；
- 独立 ownership；
- 先显示 plan；
- 明确回滚；
- 不修改未知配置；
- 不随节点卸载自动删除，除非用户对该模块单独确认；
- 不把“建议值”伪装成所有线路通用优化。

## 2. 系统检查

```bash
vpskit system inspect
```

输出：

- OS、架构、内核；
- CPU、内存、Swap；
- 磁盘和 inode；
- systemd、时间同步、DNS；
- IPv4/IPv6；
- 当前拥塞控制和 qdisc；
- TCP/UDP 监听；
- UFW/firewalld/nftables；
- cloud security group 人工检查提示；
- reboot-required；
- Xray/sing-box/WARP RSS；
- UDP buffer 和网络错误计数。

只读检查可先于系统写模块实现。

## 3. 原生 BBR

```bash
vpskit system congestion status
vpskit system congestion plan-bbr
vpskit system congestion enable-bbr --plan-id <id>
vpskit system congestion rollback
```

边界：

- 只使用当前发行版内核已有 BBR；
- 不替换内核；
- 使用 VPSKit 专属 sysctl drop-in；
- 记录原值；
- 变更后验证实际生效值；
- 需要重启时只提示，不自动重启；
- 不保证 BBR 一定比当前算法更快。

## 4. Swap

```bash
vpskit system swap status
vpskit system swap plan --size 512M
vpskit system swap create --plan-id <id>
vpskit system swap remove --plan-id <id>
```

规则：

- 默认不创建；
- 检查磁盘、文件系统和现有 Swap；
- 512MB 只是 1GB/10GB 机器的候选，不是固定值；
- 使用显式受管文件；
- 记录 fstab 变更和原始状态；
- 节点卸载不自动删除；
- 删除前确认不再使用且不是唯一系统 Swap。

## 5. 防火墙

```bash
vpskit firewall inspect
vpskit firewall plan
vpskit firewall apply --plan-id <id>
vpskit firewall rollback
```

首批 stable：

- active UFW；
- active firewalld；
- manual/noop。

nftables 自动写入保持 experimental，直到验证：

- 专用 table/chain；
- hook priority；
- 现有 default drop；
- Docker/UFW 共存；
- IPv4/IPv6；
- 重启持久化；
- 精确卸载；
- Hysteria2 端口跳跃 redirect。

云安全组不由 VPSKit 自动修改，只输出准确端口/协议清单。

## 6. UDP 调优

适用于 Hysteria2：

- 读取 socket buffer 和 `net.core.rmem_max/wmem_max`；
- 根据内存和目标吞吐生成建议；
- 可选写入受管 sysctl；
- 不盲目使用超大社区模板；
- 记录修改前后值和压力测试；
- 失败恢复原值。

## 7. Hysteria2 能力与当前版本门

当前生产基线为 `sing-box v1.13.14`。必须把能力分成以下层次：

| 功能            | 当前核心判断                                                 | 实现前置                                  |
| ------------- | ------------------------------------------------------ | ------------------------------------- |
| Salamander    | sing-box 已有基础字段                                        | 服务端、分享链接和所有目标客户端验证                    |
| 客户端端口跳跃       | sing-box outbound 自 1.11 有 `server_ports/hop_interval` | 客户端 Renderer 支持                       |
| 服务端端口范围       | 当前 sing-box inbound 没有等价范围监听字段                         | VPSKit 防火墙 redirect 或评估独立 Hysteria 核心 |
| Gecko         | 上游 sing-box 1.14.0 变化                                  | 先完成核心升级与客户端矩阵                         |
| 随机跳跃上限        | 上游 sing-box 1.14.0 `hop_interval_max`                  | 核心和客户端均升级                             |
| `bbr_profile` | 上游 sing-box 1.14.0                                     | 独立性能/CPU/丢包基准                         |
| Realm         | 上游 sing-box 1.14.0                                     | 实验功能，不适合普通公网 VPS 优先实现                 |

### 7.1 端口跳跃的安全实现

当前 VPSKit 不应为了端口跳跃给 sing-box 服务进程增加 `CAP_NET_ADMIN`。

优先设计：

```text
sing-box 继续监听单个受管 UDP 端口
VPSKit firewall module 建立专用 redirect 规则
客户端使用端口范围
```

要求：

- 端口范围显式显示；
- 云安全组由用户人工同步；
- IPv4/IPv6 规则一致；
- ownership 精确；
- 重启后持久化；
- 卸载只删除 VPSKit 创建的规则；
- 客户端不支持时不输出范围。

官方 Hysteria 核心的 Linux 端口范围会自行操作 nftables/iptables并需要相应权限；VPSKit 当前使用 sing-box，不得把该能力直接视为现有服务端实现。

## 8. Hysteria2 性能功能

- 不根据单次延迟测试自动修改拥塞控制；
- 记录 direct/Hy2/Reality 的 RTT、吞吐、丢包、CPU 和 RSS；
- 分开测试短连接、长下载和抖动链路；
- 提供恢复默认；
- 只有目标客户端支持相同字段时才写入订阅；
- Hy2 可作为性能优先组首选，但不是全网条件下的绝对最快节点。

## 9. 新协议顺序

### AnyTLS

优先级最高的新协议候选：

- sing-box 1.12+ 有正式入站；
- 可复用 sing-box 和证书；
- TCP 路径与 Hy2 互补；
- 必须先完成 `instances[]`、Renderer capability 和目标客户端矩阵；
- 不进入默认 balanced Profile。

### XHTTP + REALITY

- 作为 Xray 传输扩展；
- 与现有 RAW + REALITY 高度重叠；
- 参数和客户端矩阵更复杂；
- 仅 experimental；
- 不与状态大迁移同一版本开发。

### TUIC

- 与 Hy2 同属 UDP/QUIC；
- 边际收益较低；
- 只有用户实际线路 A/B 明显优于 Hy2 时再加入；
- 默认关闭 0-RTT 等高风险设置，按上游安全建议执行。

## 10. Fail2ban

可选模块，仅保护有可靠日志语义的服务，例如：

- SSH；
- 未来内置订阅 HTTPS 服务；
- 其他明确文本日志来源。

不对 Reality/Hy2 认证失败做未经验证的自动封禁，避免误封和日志放大。

## 11. 明确不加入

- BBR Plus、锐速、第三方内核；
- 一键 DD；
- 自动修改 SSH 端口；
- 自动关闭密码登录；
- 默认安装 Docker；
- 默认 Web 面板；
- 未固定版本的远程优化脚本；
- 未经 plan 直接重写 sysctl、nftables 或 fstab。

---

# 08｜版本路线图与优先级

> 标题：VPSKit 版本路线图与优先级
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：将功能演进拆成可发布、可回滚、可验收的垂直版本切片

## 1. 路线调整原则

原方案将架构重构放到 v0.7，晚于订阅、多客户端、恢复和 WARP。本次改为“从下一补丁版本开始切接口，第三协议前完成实例迁移”。

不做：

- 一次性大重构；
- 同一版本同时引入状态大迁移、多个客户端和新协议；
- 为追求版本号把 experimental 标成 stable。

## 2. v0.1.1：Renderer 与运维基础

目标：不改变现有双核心部署语义，建立后续扩展切缝。

功能：

- 节点 `node_id`、显示名、地区和提供商元数据；
- Mihomo 结构化 YAML；
- Renderer interface、capability 和 compatibility profile；
- 现有 Mihomo/sing-box/link Renderer 接入统一 Artifact Set；
- static Publisher；
- cleanup plan/apply；
- allowlist support bundle；
- `system inspect` 只读子集；
- 本地准备材料安全整改。

发布门：

- schema 5 老状态迁移/回退；
- 当前客户端输出参数零意外漂移；
- 固定 Mihomo/sing-box/Xray 解析通过；
- Debian 13 amd64 实机升级和回滚；
- 现有 Reality/Hy2 GUI 回归。

## 3. v0.2.0：单 VPS 自动订阅 MVP

目标：从静态导出升级为持续交付，但保持最小范围。

功能：

- Workers Publisher；
- 单 VPS 节点级发布入口；
- Cloudflare 管理 Token 与 VPS 发布凭据分离；
- Mihomo 完整配置订阅；
- v2rayN 节点订阅；
- ETag/Last-Modified/修订头；
- read-token 轮换和吊销；
- KV 最终一致性状态与回读；
- static 后备导出。

不包含：

- Loon/Shadowrocket stable；
- 多 VPS；
- WARP；
- 新协议。

## 4. v0.2.1：分流、DNS 与去广告

目标：节点、策略组、DNS、分流和广告规则共同交付。

功能：

- `minimal`；
- `cn-direct`；
- `cn-direct-antiad`；
- `ai-enhanced`；
- 白名单和自定义规则；
- 标准/严格广告模式；
- 公共版本化规则资源；
- `ruleset_revision`；
- 来源锁、许可证记录和规则回滚；
- v2rayN 路由导入实验支持。

## 5. v0.3.0：可移植加密快照

目标：重装后支持全新部署或安全恢复。

功能：

- age 加密 portable snapshot；
- inspect/plan/apply；
- identical/rotate/regenerate/config-only；
- 跨架构只恢复状态并重新下载二进制；
- 证书重新校验/签发；
- read/publisher token 轮换；
- 快照篡改和错误口令测试。

## 6. v0.4.0：更多客户端与少量多 VPS

目标：在订阅单节点闭环稳定后扩展客户端和聚合。

功能：

- Loon Renderer 和实机门；
- Shadowrocket Renderer 和实机门；
- Android 目标客户端明确化；
- 2～10 台节点聚合；
- 每台 VPS 节点级发布凭据；
- 聚合策略组和命名冲突处理；
- 故障节点禁用；
- schema 7 `instances[]` 迁移完成。

## 7. v0.5.0：Hysteria2 增强

优先在当前 pin 上评估：

- Salamander；
- 客户端端口跳跃；
- VPSKit 防火墙 redirect；
- UDP buffer 检查；
- 性能、CPU、RSS 和丢包基准。

若升级 sing-box 1.14+，另设核心升级子版本和回滚门，再评估：

- Gecko；
- 随机跳跃区间；
- `bbr_profile`；
- Realm experimental。

## 8. v0.6.0：WARP 与 AI 出站

功能：

- 官方 Linux WARP 固定版本；
- Local Proxy 能力探测；
- `ai-only` 和 `custom`；
- availability-first/privacy-first；
- direct/WARP A/B；
- 故障回退；
- AI 规则包；
- 资源和重启验证。

## 9. v0.7.0：新协议

前提：通用实例模型、Renderer capability、迁移和客户端矩阵已稳定。

顺序：

1. AnyTLS experimental → stable；
2. XHTTP + REALITY experimental；
3. TUIC 仅在实线 A/B 有价值时加入。

每个协议单独版本，不一次加入多个。

## 10. v0.8.0：系统模块

- UFW/firewalld；
- 原生 BBR；
- Swap；
- UDP sysctl；
- 系统更新辅助；
- reboot-required；
- Fail2ban；
- 网络质量测试。

系统模块不成为节点安装前置。

## 11. v1.0：个人 VPS 综合管理版

验收要求：

- 所有 stable 功能均有固定版本和实机证据；
- Debian 12/13、Ubuntu 24.04 的目标路径分级验证；
- amd64 主路径和 arm64 主要路径；
- 1C1G 长期稳定和 10GB 磁盘增长证据；
- 自动订阅、规则、恢复和回滚闭环；
- 多客户端兼容矩阵真实可复现；
- 高风险模块默认关闭；
- 迁移、卸载、灾备和安全文档完整。

## 12. 每个版本的统一原则

- 一次 Release 只解决一个主要风险域；
- 先实现 rollback，再开放菜单入口；
- experimental 不进入默认安装；
- 每个新供应链来源必须固定版本、摘要和许可证；
- 每个 Renderer 必须绑定 compatibility profile；
- 当前 VPS 和客户端继续可用是最高回归门；
- 版本完成取决于验收证据，不取决于功能代码已合并。

---

# 09｜测试验收与发布门禁

> 标题：VPSKit 测试验收与发布门禁
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：为架构、订阅、规则、恢复和可选模块定义可复核的发布证据

## 1. 证据分级

文档中的“支持”必须对应以下证据之一，不能用目标规划替代测试结论：

| 等级  | 含义                                 | 可使用的措辞        |
| --- | ---------------------------------- | ------------- |
| E0  | 仅设计或官方能力调研                         | 计划、候选、待验证     |
| E1  | 单元测试、静态检查或 Renderer golden test 通过 | 实现完成，尚未实机     |
| E2  | 固定版本客户端成功解析或导入                     | 已通过格式验证       |
| E3  | 固定系统/架构的端到端部署、连接、更新和回滚通过           | 已在指定矩阵验证      |
| E4  | 低资源长时间运行和故障注入通过                    | 可进入 stable 候选 |

每条验收记录必须写明：版本、系统、架构、命令或步骤、期望值、实际值、日志位置和结论。

## 2. 当前基线与未验证边界

截至本次审查：

- 公共仓库主分支为提交 `22b6457db557b3fc3e4b723784e33d6d55f563fe`；
- Debian 13 amd64 已完成 v0.1.0 第一阶段端到端验证；
- GitHub Actions 对 Go、生成配置、Shell 和密钥扫描为成功状态；
- Debian 12、Ubuntu 24.04、arm64 实机、1C1G 长时间运行、Loon、Shadowrocket、WARP 和便携快照仍未获得 E3/E4 证据；
- v0.2.0 单 VPS 自动订阅已获得 E3 证据：Cloudflare 预发布/正式 Worker/KV 健康检查、节点级发布、远端回读、ETag/HEAD/304、Token 轮换/吊销、Debian 13 amd64 自动发布和 Win11 Clash Verge Rev/v2rayN 订阅更新均已通过；
- 本次只修改方案文档，没有据此把未实现能力标为“已支持”。

## 3. Renderer 与客户端门禁

### 3.1 通用测试

每个 Renderer 必须具有：

1. 正常输入 golden test；
2. 缺字段、错误类型、超长名称和特殊字符测试；
3. IPv4、IPv6、域名三种服务器地址测试；
4. 同名节点去重和稳定排序测试；
5. 不输出未声明 capability 的字段；
6. 输出中不得包含发布凭据、恢复口令或服务端私钥；
7. 固定版本解析器或客户端的导入测试；
8. 配置修订后可更新，旧配置仍能给出明确失败或迁移提示。

### 3.2 客户端矩阵

| 客户端                      | 固定测试基线                                                | 第一目标          | 进入 stable 前的最低证据                      |
| ------------------------ | ----------------------------------------------------- | ------------- | ------------------------------------- |
| Mihomo / Clash Verge Rev | Windows 11；Clash Verge Rev `v2.5.2`；Mihomo `v1.19.29` | 完整配置、策略组、远程规则 | 固定 Mihomo 核心解析 + Windows 实机导入、更新、规则命中 |
| v2rayN Windows           | Windows 11；v2rayN `v7.23.1`                           | 节点订阅；路由规则独立导入 | 固定版本导入、更新和 Reality/Hysteria2 连接       |
| Android Clash 类客户端       | 待提供具体应用、包名和核心版本                                       | Mihomo 完整配置   | Android 实机验证                          |
| v2rayN Android           | 待提供版本                                                 | 节点订阅          | 固定版本导入、更新和连接；不得套用 Windows 结论          |
| Loon                     | 待提供 iOS 与 Loon 版本                                     | 节点/配置/规则入口    | 真实设备导入、更新和规则命中                        |
| Shadowrocket             | 待提供 iOS 与客户端版本                                        | 实验性完整配置或节点入口  | 真实设备验证；无证据时保持 experimental            |

“固定测试基线”只表示版本选择已经完成。只有产生解析、导入、更新、规则命中和实际连接记录后，才提升证据等级。

## 4. 自动订阅门禁

### 4.1 发布端

- 节点只能持有 `node_id` 级发布凭据，不能持有 Cloudflare 账户级 KV 管理 Token；
- 发布接口校验节点身份、最大请求体、内容类型、修订号、时间窗和允许的 artifact 类型；
- 同一 `node_id + revision + content_hash` 重试必须幂等；
- 旧修订不能覆盖新修订；
- 日志只记录 Token 指纹或末尾少量字符，禁止完整 URL 和完整凭据；
- 凭据吊销后，旧凭据发布立即失败，读取端仍能读取最后一个有效修订；
- 发布失败不得删除当前有效配置。

### 4.2 读取端

- 仅允许 `GET`、`HEAD`；其他方法返回明确状态；
- 随机读取 Token 不得枚举或推断其他订阅；
- 返回正确的 `Content-Type`、`ETag`、`Last-Modified`、修订号和合理缓存头；
- `If-None-Match` 命中时返回 `304`；
- Token 轮换支持限定时间的双 Token 过渡并可立即撤销；
- `/auto` 识别失败时不能返回错误格式，应提示用户使用显式客户端路径；
- 读取日志不得保存完整路径 Token。

### 4.3 Workers KV 一致性测试

Workers KV 是最终一致，不以跨 key 原子事务作为设计前提。必须验证：

- 写入新修订后，不同区域只会看到“完整旧修订”或“完整新修订”；
- manifest 最后发布，artifact 使用不可变的修订路径；
- 缺少任一 artifact 时不切换 manifest；
- 超时窗口内读取旧修订属于可接受状态，但混合修订属于失败；
- 回滚通过发布新的 manifest 指针完成，不能原地拼接覆盖多份内容。

## 5. 规则、DNS 与去广告门禁

- 每个规则源记录仓库、固定提交或 Release、文件路径、许可证、摘要和抓取时间；
- 构建期锁定源版本，运行时不从未知 `main` 直接更新；
- domain、ipcidr、classical 与 MRS 格式分别进行解析测试；
- 域名规则执行前，DNS 模式必须能提供真实域名或可靠映射；
- Fake-IP、redir-host、IPv6、DoH/DoT 和 DNS 泄漏分别验证；
- 广告拦截需用命中样例和误杀回归清单测试；
- 白名单必须优先于第三方广告规则；
- 公共规则 URL 不复用私有节点订阅 Token；
- 上游不可用时继续使用最后一个通过校验的规则版本。

## 6. 便携快照与恢复门禁

### 6.1 快照生成

- 明文暂存只存在于 `0700` 临时目录，文件权限为 `0600`；
- 成功或失败退出都清理明文暂存；
- 口令不通过命令行参数、日志或 shell history 传递；
- 快照含 schema、VPSKit 版本、创建时间、来源主机摘要、内容清单和每项哈希；
- 默认不含日志、缓存、核心二进制和无关系统文件；
- 检查命令在不解密秘密内容的情况下显示公开 envelope。

### 6.2 恢复

- `inspect`、`plan`、`apply` 三段式；
- 错误口令、损坏文件、旧 schema、新 schema 和部分文件缺失均安全失败；
- 恢复前自动生成现有状态回滚点；
- 相同架构和跨架构恢复只复用状态与凭据，核心二进制按目标平台重新下载和校验；
- 支持“保留原凭据”和“重新生成协议凭据”两条路径；
- 发布身份是否保留必须单独选择，不默认复制长期管理凭据；
- 故障注入后能恢复到恢复前状态，或给出明确人工处置说明。

## 7. Hysteria2、WARP 与系统模块门禁

### Hysteria2

- 每个高级字段与锁定 sing-box 版本对应；
- `server_ports`/`hop_interval` 仅作为客户端能力处理，服务端跳跃范围必须有 VPSKit 管理的防火墙计划、应用和回滚；
- 不为 sing-box/Xray 核心授予 `CAP_NET_ADMIN`；
- 云安全组和主机防火墙端口范围分别检查；
- UDP 不可用时 Reality 仍可连接。

### WARP

- 启用前记录官方客户端版本、模式和资源基线；
- 只影响选择的 AI/custom 规则，不改变 SSH、系统默认路由和入站监听；
- direct 与 WARP 出口分别测试 IP、地区、DNS、延迟和目标服务可用性；
- 目标服务拒绝、WARP 断线和本地代理退出时按用户选择 fail-open 或 fail-closed；
- 卸载后无残留路由、服务、仓库和配置。

### 系统模块

- 所有修改使用 VPSKit 专属 drop-in；
- `plan` 输出具体文件、命令和预期重启影响；
- `apply` 后读回内核/服务实际状态；
- `rollback` 只撤销 VPSKit 自己创建的对象；
- 不替换内核，不自动修改 SSH，不覆盖用户防火墙规则。

## 8. 低资源与破坏性测试

目标矩阵至少包含：

- 1 vCPU / 1GB RAM / 10GB 磁盘；
- 空闲 24 小时、持续传输、配置发布、规则更新、系统升级和磁盘接近阈值；
- 记录 Xray、sing-box、可选 WARP、订阅发布时的 RSS、CPU、文件数和日志增长；
- 模拟网络中断、DNS 故障、证书续期失败、磁盘满、进程被杀、发布端不可达、KV 延迟和错误快照；
- 验证每个失败点是否保持旧配置可用、是否产生可读诊断、是否可回滚。

## 9. 分阶段发布门禁

| 阶段     | 必须通过                                                           |
| ------ | -------------------------------------------------------------- |
| v0.1.1 | 现有双协议回归、结构化 Renderer、golden、migrate check/plan、cleanup dry-run |
| v0.2.0 | 单 VPS Mihomo 订阅、发布凭据隔离、KV 完整修订、Token 轮换、静态导出兜底                 |
| v0.2.1 | DNS/规则/去广告、来源锁定、误杀回归、公共规则与私有订阅分离                               |
| v0.3.0 | 加密快照、跨重装恢复、故障注入、明文零残留                                          |
| v0.4.0 | 真实客户端矩阵、多 VPS 聚合、节点级隔离、状态 schema 迁移                            |
| v0.5+  | 对应高级功能的版本门禁、资源预算、卸载和回滚证据                                       |
| v1.0   | 所有 stable 能力达到 E3；核心路径达到 E4；文档、安装包与证据一致                        |

## 10. 发布证据包

每次正式发布保存：

```text
release-evidence/
├── manifest.json
├── environment.json
├── checksums.txt
├── unit-and-golden-tests.txt
├── client-matrix.md
├── e2e-results.md
├── rollback-results.md
├── resource-observations.csv
└── known-limitations.md
```

最终原则：无法提供证据的能力只能标为 E0/E1 或 experimental；“官方支持某字段”不能替代 VPSKit 在锁定版本、目标系统和真实客户端上的验证。

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

| 能力                                               | 上游版本信息     | 对当前 `v1.13.14` 的结论   |
| ------------------------------------------------ | ---------- | -------------------- |
| Hysteria2 outbound `server_ports`、`hop_interval` | 1.11 起     | 可用于支持该字段的客户端输出，但仍需实测 |
| AnyTLS                                           | 1.12 起     | 上游具备；VPSKit 未实现      |
| Gecko、`bbr_profile`、Realm、部分跳跃增强                 | 1.14 文档/变更 | 当前锁定版本不能直接启用         |

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
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：记录本次审查范围、问题、处置和未执行事项

## 1. 审查范围

本次交叉审查了：

- 本地 `VPSKit-功能演进完整方案-v0.3-20260720` 原始分卷、完整版和 ZIP；
- 公共仓库 `filence/vpskit` 的主分支源码、提交、文档和 GitHub Actions；
- 用户提供的“VPSKit 功能建议”对话；
- Cloudflare、Mihomo、sing-box、Hysteria 2、v2rayN、Loon 和 WARP 的官方资料；
- 项目工作区中与前期环境相关的本地忽略文件；凭据可在用户授权的 VPSKit/VPS/Cloudflare 操作范围内直接使用，但不复制其值到方案包。

审查基线：

- 日期：2026-07-21；
- VPSKit 提交：`22b6457db557b3fc3e4b723784e33d6d55f563fe`；
- 方案修订：v0.3-R1；
- 首批客户端测试基线：Windows 11；Clash Verge Rev `v2.5.2`；Mihomo `v1.19.29`；v2rayN `v7.23.1`；
- Cloudflare 准备状态：Active Zone、`sub`/`sub-dev` 候选名称、DNS 无冲突和 Workers/KV 控制台访问已由用户确认；尚未创建 Worker、KV 或订阅 DNS；
- 修改范围：方案文档、完整版、清单和方案 ZIP；未修改 Go/Bash/CI 源码。

## 2. 总体结论

原方案的产品方向成立：自动订阅、多客户端、规则与去广告、恢复、多 VPS、Hysteria2 增强、WARP 和系统工具符合个人自用场景。

但原方案不能直接作为实现规格，主要原因是：架构兑现过晚、Cloudflare 权限隔离与一致性描述不准确、上游最新版能力与当前锁定版本混用、客户端兼容结论超前、规则/DNS/许可证和秘密材料准备不足。

因此本次结论为：

> 方向通过；按 v0.3-R1 的顺序、权限模型、证据门禁和前期准备清单修订后再进入开发。

## 3. 主要问题与修改

| 级别  | 原方案问题                                   | 风险                                        | v0.3-R1 修改                                                    |
| --- | --------------------------------------- | ----------------------------------------- | ------------------------------------------------------------- |
| P0  | Adapter/Renderer/通用状态重构排到 v0.7          | 订阅、多客户端、恢复会继续绑定固定 Reality/Hy2 状态          | v0.1.1 先切 Renderer/Publisher；第三协议前完成 schema 7 通用实例迁移          |
| P0  | 计划让 VPS 直接使用 KV 写权限，并宣称单节点泄露不能修改其他节点    | Cloudflare KV 写权限为账户级，结论不成立               | 引入 node-scoped 发布入口；Cloudflare 管理 Token 不上 VPS                |
| P0  | 使用“原子切换 current revision”描述 KV 发布       | Workers KV 最终一致且无跨 key 原子事务               | 使用不可变 revision artifact，manifest 最后发布；只允许完整旧/新修订              |
| P0  | 初次审查把 Agent 读取本地忽略凭据误判为凭据外泄             | 不必要地阻塞用户已授权的直接执行                          | 明确本地文件是授权凭据入口；允许在任务范围内直接使用，不回显、不入包、不扩大权限；只有真实泄露或生命周期条件触发时轮换   |
| P1  | 直接规划 Gecko、`bbr_profile`、Realm 等 Hy2 功能 | 当前锁定 sing-box 1.13.14 不具备全部 1.14 能力       | 增加上游版本门禁；核心升级与功能启用分开发布                                        |
| P1  | 把官方 Hysteria 服务端端口范围直接映射到当前 sing-box    | 实现不同，可能需要过大网络权限                           | 明确 inbound 差异；由 VPSKit 管理防火墙重定向，不给核心 `CAP_NET_ADMIN`          |
| P1  | 多客户端均进入近期稳定目标                           | Loon/Shadowrocket/Android Clash 缺固定版本实机证据 | Mihomo 为首个 stable；v2rayN 分阶段；Loon/Shadowrocket 先 experimental |
| P1  | 规则订阅未完整处理 DNS、许可证和 Token 边界             | 规则不命中、泄漏私有订阅、再分发风险                        | 新增 DNS Profile、来源锁定、许可证记录、公共规则 URL 与私有订阅分离                    |
| P1  | 便携快照与当前本地备份边界模糊                         | 跨机恢复、二进制和凭据处理不清                           | 明确 backup 与 snapshot；快照只带状态/凭据/证书/清单，核心按目标平台重装                |
| P2  | 只有功能列表，缺用户准备清单                          | 实现时反复索要高权限凭据或无法验收                         | 新增第 11 卷，定义工具、VPS、Cloudflare、客户端和产品决策                         |
| P2  | “支持”与“计划”混用                             | 读者会误认为当前已有订阅和规则能力                         | 增加 E0–E4 证据等级和分阶段发布门禁                                         |

## 4. 保留的核心决策

以下原方案判断经审查后继续保留：

- Reality 作为 TCP 稳定备用，Hysteria2 作为性能优先节点；
- 自动订阅优先于继续堆协议；
- 不把规则塞入单个节点分享链接；
- Mihomo 完整配置订阅先行；
- WARP 只做可选精确出站，不接管系统全局路由；
- 同时支持全新生成和加密恢复两条重装路径；
- 系统工具保持低优先级、plan/apply/rollback 和专属 drop-in；
- 不加入多租户计费、公共注册、默认 Docker、Web 面板和第三方魔改内核；
- 所有新模块默认关闭、可卸载、可回滚，并有静态导出兜底。

## 5. 新的实施顺序

```text
Renderer/Publisher 与迁移基础
→ 单 VPS Mihomo 自动订阅
→ DNS、分流与去广告
→ 加密便携快照
→ 更多客户端与少量多 VPS
→ Hysteria2 高级功能
→ WARP AI 精确出站
→ 新协议
→ 系统工具
→ v1.0 证据收口
```

这个顺序不是缩减最终功能，而是让每批功能都能在较小状态迁移和权限范围内独立验收。

## 6. 本次未执行事项

本次没有：

- 修改 VPSKit 源码、版本锁、GitHub Actions 或 Release；
- 登录、变更或重装任何 VPS；
- 调用、验证或复制本地文件中的密码和 Token；
- 创建 Cloudflare Worker、KV、DNS 记录或 API Token；
- 在客户端导入真实节点；
- stage、commit、push 或创建 Pull Request；
- 宣称 Go 测试在本地通过——当前终端未发现 Go。

## 7. 后续执行建议

1. 用户确认第 11 卷的凭据授权范围和 R0 选择；现有凭据可直接用于范围内操作；
2. 把 v0.1.1 拆成独立实现计划和验收清单；
3. 先用无秘密 fixture 建立 Renderer golden tests；
4. 设计 node-scoped Publisher 协议和威胁模型，再创建 Cloudflare 权限；
5. 订阅 MVP 只做单 VPS + Mihomo + 静态导出兜底；
6. MVP 达到 E3 后再加入规则包、第二客户端和多 VPS；
7. 每个阶段同步更新 README、分卷、完整版、安装包说明和发布证据。

## 8. 修订物清单

v0.3-R1 包含：

- 00–10：重写或校正原方案内容；
- 11：新增前期材料与环境准备；
- 12：新增审查记录与修订说明；
- README：更新阅读顺序、证据边界和安全提示；
- 单文件完整版：由分卷按顺序重新生成；
- FILE-MANIFEST：记录最终文件大小和 SHA-256；
- ZIP：从最终白名单文件重建并逐项校验。

最终接受标准：分卷、完整版、README、清单和 ZIP 内容一致；授权凭据只从本地忽略文件按需读取，包内不包含 `前期环境须知.md`、密码、API Token、私钥或其他真实秘密。
