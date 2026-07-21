# VPSKit 功能演进完整方案包

- 方案版本：v0.3
- 审查修订：R3（实施状态校正）
- 原编制日期：2026-07-20
- 本次修订日期：2026-07-21
- 对应项目：[filence/vpskit](https://github.com/filence/vpskit)
- 当前产品基线：VPSKit `v0.2.1-lab.4`；方案 A 已完成 Windows 11 Clash Verge Rev r0007 验收，`doctor --fix` 与扩展 `system inspect` 已完成 Debian 13 amd64 实机验收
- 使用对象：开发者个人自用、少量 VPS、低资源环境

## 本次精简结论

后续只保留以下主线：

```text
通用实例/Adapter/Renderer
→ 结构化 Mihomo 与节点元数据
→ ACL4SSR + anti-AD 方案 A/B、DNS、规则自动更新（方案 A 已验收；来源固定待完成）
→ doctor --fix 与 system inspect（已验收）
→ Hysteria2 强化与 UDP 调优
→ Fail2ban 与系统健康检查
```

规则默认是方案 A：ACL4SSR + anti-AD + fake-ip DNS + Sniffer；发生 anti-AD 误杀时可切换方案 B：仅 ACL4SSR + fake-ip DNS + Sniffer。当前 Rule Provider 直连上游并由 Mihomo 缓存，尚未具备来源固定或 VPSKit 镜像缓存。

加密快照、多 VPS、更多 iOS Renderer、WARP、新协议、通用 BBR/Swap/防火墙和面板从活跃路线移除，未来需要时再独立评估。

## 文档目录

1. [00-执行摘要与决策清单.md](00-执行摘要与决策清单.md)
2. [01-现状审计与产品边界.md](01-现状审计与产品边界.md)
3. [02-目标架构与扩展模型.md](02-目标架构与扩展模型.md)
4. [03-自动订阅与多客户端交付.md](03-自动订阅与多客户端交付.md)
5. [04-分流规则与去广告方案.md](04-分流规则与去广告方案.md)
6. [05-WARP与AI出站方案.md](05-WARP与AI出站方案.md)（暂停）
7. [06-恢复多VPS与运维闭环.md](06-恢复多VPS与运维闭环.md)（暂停，仅保留既有 `doctor` 参考）
8. [07-系统工具与高级功能.md](07-系统工具与高级功能.md)
9. [08-版本路线图与优先级.md](08-版本路线图与优先级.md)
10. [09-测试验收与发布门禁.md](09-测试验收与发布门禁.md)
11. [10-参考项目与资料.md](10-参考项目与资料.md)
12. [11-前期材料与环境准备.md](11-前期材料与环境准备.md)
13. [12-审查记录与修订说明.md](12-审查记录与修订说明.md)
14. [VPSKit-完整方案-v0.3-20260720.md](VPSKit-完整方案-v0.3-20260720.md)

## 不变的边界

- 新模块默认关闭、独立状态、可回滚和可卸载；
- 不在方案、Git、ZIP 或回复中写入密码、Token、私钥或订阅 URL；
- 规则仅做域名/IP 分流与 REJECT，不做 MITM、用户 CA、HTTPS 解密或脚本改写；
- 未经过目标客户端实测的能力不得标记为 stable；
- 订阅和规则更新失败时保留上一完整修订与客户端缓存。
