# VPSKit 方案包文件清单

- 方案版本：v0.3-R19
- 清单生成日期：2026-07-22
- 工作区基线：v0.2.9-lab.1，方案 A r0008、方案 B r0014、Salamander r0015、schema 9 r0012、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线与受管 UDP buffer 实机验收
- 说明：为避免自引用，清单不记录自身哈希；ZIP 仍包含本清单。

| 文件 | 字节数 | SHA-256 |
| --- | ---: | --- |
| `00-执行摘要与决策清单.md` | 5341 | `95a7d6bc091cdf80113add8a2a2af1eb623eb5096758b1e8781f51f8f7cf7ae2` |
| `01-现状审计与产品边界.md` | 6676 | `db49ec0679631de41c9871a44fa1db1f2e306a16c707e3d6b2043d218db666b1` |
| `02-目标架构与扩展模型.md` | 7551 | `befce79b403e37acfe00f5ed088641b0883d7ed339234716ba0688d014a8b19b` |
| `03-自动订阅与多客户端交付.md` | 10172 | `ef0bcd1d9b91b38ff82046798cf048ebb5ced7f83651af4ed1662b6048a22494` |
| `04-分流规则与去广告方案.md` | 6205 | `3ccce01c369bcbc8aa06307a1f29b9e490e6cd662759cc7f18f170636ca72891` |
| `05-WARP与AI出站方案.md` | 663 | `6e88e0809eb7eec0bd6aed61f908eb7c4fcab48b7e6bbc80f6bd4e22d782451e` |
| `06-恢复多VPS与运维闭环.md` | 579 | `0f6a60ca2edc03937ed6ccf75a9f1f15a6480fd3149691d45564375278510549` |
| `07-系统工具与高级功能.md` | 8982 | `ed69fcfba4be97ae10143d7b750315dae285100e735c906e9443b9b19b20ed5f` |
| `08-版本路线图与优先级.md` | 3078 | `59fae664d5c16a70a6509327d3da2d8eacddc7ded8d7f28478af0a7becc87c0d` |
| `09-测试验收与发布门禁.md` | 2505 | `537f9d044ee19ca013bcb43b901aebefc399bf93e4c70d9206f87dcfebe7ee6e` |
| `10-参考项目与资料.md` | 7086 | `85cd8ccf48b1ac8b2d441ba1a3d50028f7f32ea8c4c067a6d7c4fa6b746dcde2` |
| `11-前期材料与环境准备.md` | 10992 | `bfcb9bd5176ed99a705d1efdd84c31ff2c0d2940f5a6813578b889e3d2bb3ab6` |
| `12-审查记录与修订说明.md` | 6515 | `d987c03aa6a41311e131b9ea60b69e8b12473e2dc0c1bd6c3fb15019b1d5c262` |
| `README.md` | 3222 | `f7759a319e51f7b3af6aa5fdd609482fe382d2c2b399f830c20fb32d86b28fa2` |
| `VPSKit-完整方案-v0.3-20260720.md` | 77343 | `83dc36590e9c2c4177a03aff2383e227e335d186c2ca18e34e321bcc578fe0eb` |

## 包内白名单

本包仅包含本方案目录内的 Markdown 文档和本清单；不包含 VPS 密码、Cloudflare Token、私钥、客户端订阅链接或任何其他凭据。
