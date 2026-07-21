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
