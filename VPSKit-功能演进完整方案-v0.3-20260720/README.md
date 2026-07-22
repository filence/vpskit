# VPSKit 功能演进完整方案包

- 方案版本：v0.3
- 审查修订：R19（Hysteria2 受管 UDP buffer 实机验收）
- 原编制日期：2026-07-20
- 本次修订日期：2026-07-22
- 对应项目：[filence/vpskit](https://github.com/filence/vpskit)
- 当前产品基线：VPSKit `v0.2.9-lab.1`；方案 A r0008、方案 B r0014 与 Salamander r0015 已完成 Windows 11 Clash Verge Rev 验收，schema 9 白名单/自定义规则已完成 Debian 13 r0012 发布回读，`doctor --fix`、扩展 `system inspect`、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线、受管 UDP buffer 与系统更新候选检查已完成实机验收
- 使用对象：开发者个人自用、少量 VPS、低资源环境

## 本次精简结论

后续只保留以下主线：

```text
通用实例/Adapter/Renderer
→ 结构化 Mihomo 与节点元数据
→ ACL4SSR + anti-AD 方案 A/B、DNS、规则自动更新（方案 A r0008、方案 B r0014 已验收；受管缓存已完成）
→ doctor --fix 与 system inspect（已验收）
→ Hysteria2 强化（UDP 调优已完成首个受管档位）
→ Fail2ban（含 SSH 白名单，已验收）与剩余系统健康检查
```

规则默认是方案 A：ACL4SSR + anti-AD + fake-ip DNS + Sniffer；发生 anti-AD 误杀时可切换方案 B：仅 ACL4SSR + fake-ip DNS + Sniffer。`rules refresh` 会将规则下载、限额检查、哈希并作为受管 Worker 工件发布；客户端不再直接访问 ACL4SSR、anti-AD 或 MetaCubeX URL。

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
