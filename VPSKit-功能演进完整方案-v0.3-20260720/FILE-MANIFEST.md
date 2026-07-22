# VPSKit 方案包文件清单

- 方案版本：v0.3-R28
- 清单生成日期：2026-07-22
- 工作区基线：v0.2.21-lab.1，方案 A r0008、方案 B r0014、Salamander r0015、schema 9 r0012、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线、受管 UDP buffer、Adapter Registry 渐进接入、服务等待收口、只读带宽建议与端口跳跃 redirect/客户端导出/重启恢复实机验收；schema 10 通用实例兼容层已完成当前 VPS 部署验收
- 说明：为避免自引用，清单不记录自身哈希；ZIP 仍包含本清单。

| 文件 | 字节数 | SHA-256 |
| --- | ---: | --- |
| `00-执行摘要与决策清单.md` | 4943 | `e0c5615812bc7560492c9d4aff83433a17cebb3068079d041388db194368a69b` |
| `01-现状审计与产品边界.md` | 7124 | `bc7c529c56fc2d32d3f8189c6582c109afac918dd7dd4a22ffe1ef207d58720b` |
| `02-目标架构与扩展模型.md` | 8533 | `dfdfaf90a0109eadb460508d5f53bfe855c3cb7fd087dab459edf30210a68911` |
| `03-自动订阅与多客户端交付.md` | 10172 | `ef0bcd1d9b91b38ff82046798cf048ebb5ced7f83651af4ed1662b6048a22494` |
| `04-分流规则与去广告方案.md` | 6327 | `f9f5de33e775e994a8a4e2a733092c7aa49f6a4d7d38952456b3c4a7a1043fb5` |
| `05-WARP与AI出站方案.md` | 663 | `6e88e0809eb7eec0bd6aed61f908eb7c4fcab48b7e6bbc80f6bd4e22d782451e` |
| `06-恢复多VPS与运维闭环.md` | 579 | `0f6a60ca2edc03937ed6ccf75a9f1f15a6480fd3149691d45564375278510549` |
| `07-系统工具与高级功能.md` | 10441 | `d675a25fe19290d47afb682e803332d833cb3998e681ce9e136f660a853f54a0` |
| `08-版本路线图与优先级.md` | 3645 | `b4afd336705042a531dc5d5bb7d310182fe704c05e90d857d554e9936a4cbc03` |
| `09-测试验收与发布门禁.md` | 2981 | `3f76739f31fbce4380b9d593726f8c51bdf547c590d12e35da975a0fbfded17b` |
| `10-参考项目与资料.md` | 7086 | `85cd8ccf48b1ac8b2d441ba1a3d50028f7f32ea8c4c067a6d7c4fa6b746dcde2` |
| `11-前期材料与环境准备.md` | 10992 | `bfcb9bd5176ed99a705d1efdd84c31ff2c0d2940f5a6813578b889e3d2bb3ab6` |
| `12-审查记录与修订说明.md` | 10518 | `a64019dfac0c4b3c66d161957ce98989c74383fd9cbb0282963f3bc0f185174f` |
| `README.md` | 3365 | `664ac6fca9e64ddb6e292cedfd3c1289e1e3ce52ff127e48db0cfaf22bd4d5e4` |
| `VPSKit-完整方案-v0.3-20260720.md` | 85129 | `3abaabe39e37f590b81c6d87147cdabbe3c76d1c6a08e316d330e547b7b63c70` |

## 包内白名单

本包仅包含本方案目录内的 Markdown 文档和本清单；不包含 VPS 密码、Cloudflare Token、私钥、客户端订阅链接或任何其他凭据。
