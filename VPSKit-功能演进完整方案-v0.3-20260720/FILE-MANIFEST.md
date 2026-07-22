# VPSKit 方案包文件清单

- 方案版本：v0.3-R20
- 清单生成日期：2026-07-22
- 工作区基线：v0.2.9-lab.1，方案 A r0008、方案 B r0014、Salamander r0015、schema 9 r0012、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线与受管 UDP buffer 实机验收；schema 10 通用实例兼容层已完成源码/单元测试，待 VPS 部署验收
- 说明：为避免自引用，清单不记录自身哈希；ZIP 仍包含本清单。

| 文件 | 字节数 | SHA-256 |
| --- | ---: | --- |
| `00-执行摘要与决策清单.md` | 5497 | `ea70d9997f92d6bbeb18059b9163793b14e69d5aced6264c44f14adea4628d4d` |
| `01-现状审计与产品边界.md` | 7119 | `e3efe96741f592ba9d1580ea82bdfcd7de1de78ff5295800b105c947fe37d86e` |
| `02-目标架构与扩展模型.md` | 8461 | `bea68758ed3353a76f16e7cfa34eb9717efa1956b175ec3470aa28e7ea0d8a1c` |
| `03-自动订阅与多客户端交付.md` | 10172 | `ef0bcd1d9b91b38ff82046798cf048ebb5ced7f83651af4ed1662b6048a22494` |
| `04-分流规则与去广告方案.md` | 6205 | `3ccce01c369bcbc8aa06307a1f29b9e490e6cd662759cc7f18f170636ca72891` |
| `05-WARP与AI出站方案.md` | 663 | `6e88e0809eb7eec0bd6aed61f908eb7c4fcab48b7e6bbc80f6bd4e22d782451e` |
| `06-恢复多VPS与运维闭环.md` | 579 | `0f6a60ca2edc03937ed6ccf75a9f1f15a6480fd3149691d45564375278510549` |
| `07-系统工具与高级功能.md` | 8982 | `ed69fcfba4be97ae10143d7b750315dae285100e735c906e9443b9b19b20ed5f` |
| `08-版本路线图与优先级.md` | 3258 | `f090b54854a4c0b7725c50026aefdd06f21a71945184b93aa4cfd0cd6a4832c0` |
| `09-测试验收与发布门禁.md` | 2784 | `4e2bb2331ba98424e4fb7cc3fa5adcfaf8b3492fd3ca2676045bc8679484dfbf` |
| `10-参考项目与资料.md` | 7086 | `85cd8ccf48b1ac8b2d441ba1a3d50028f7f32ea8c4c067a6d7c4fa6b746dcde2` |
| `11-前期材料与环境准备.md` | 10992 | `bfcb9bd5176ed99a705d1efdd84c31ff2c0d2940f5a6813578b889e3d2bb3ab6` |
| `12-审查记录与修订说明.md` | 7457 | `af29d0c8f6173cdb16486012242c1cdccce45a65f9488618568c95c3c40422b5` |
| `README.md` | 3317 | `84ef11addfe002d0474fe4816502b3dd4cca07f977bc1dd7b0587442279d942f` |
| `VPSKit-完整方案-v0.3-20260720.md` | 80338 | `13329dce0daca8d1ea934aaab61d9402559fb7060b51058e50a7a33342661154` |

## 包内白名单

本包仅包含本方案目录内的 Markdown 文档和本清单；不包含 VPS 密码、Cloudflare Token、私钥、客户端订阅链接或任何其他凭据。
