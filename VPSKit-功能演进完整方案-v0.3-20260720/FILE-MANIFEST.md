# VPSKit 方案包文件清单

- 方案版本：v0.3-R23
- 清单生成日期：2026-07-22
- 工作区基线：v0.2.13-lab.1，方案 A r0008、方案 B r0014、Salamander r0015、schema 9 r0012、Fail2ban SSH 白名单、规则刷新失败保护、Hysteria2 能力矩阵、性能基线、受管 UDP buffer、Adapter Registry 渐进接入与只读带宽建议实机验收；schema 10 通用实例兼容层已完成当前 VPS 部署验收
- 说明：为避免自引用，清单不记录自身哈希；ZIP 仍包含本清单。

| 文件 | 字节数 | SHA-256 |
| --- | ---: | --- |
| `00-执行摘要与决策清单.md` | 5683 | `f3c8ceee245a0136b9c6c5c7c288317fd92cfc06ebff8760a306d994f37d53fa` |
| `01-现状审计与产品边界.md` | 7124 | `bc7c529c56fc2d32d3f8189c6582c109afac918dd7dd4a22ffe1ef207d58720b` |
| `02-目标架构与扩展模型.md` | 8533 | `dfdfaf90a0109eadb460508d5f53bfe855c3cb7fd087dab459edf30210a68911` |
| `03-自动订阅与多客户端交付.md` | 10172 | `ef0bcd1d9b91b38ff82046798cf048ebb5ced7f83651af4ed1662b6048a22494` |
| `04-分流规则与去广告方案.md` | 6205 | `3ccce01c369bcbc8aa06307a1f29b9e490e6cd662759cc7f18f170636ca72891` |
| `05-WARP与AI出站方案.md` | 663 | `6e88e0809eb7eec0bd6aed61f908eb7c4fcab48b7e6bbc80f6bd4e22d782451e` |
| `06-恢复多VPS与运维闭环.md` | 579 | `0f6a60ca2edc03937ed6ccf75a9f1f15a6480fd3149691d45564375278510549` |
| `07-系统工具与高级功能.md` | 9891 | `27bd16c88872711b530fbdd18f3916b71ea70fc912a33e03a7e28160fd6223d5` |
| `08-版本路线图与优先级.md` | 3394 | `eb7122bac1a3e3b767baeafceafb5bb529f6075a21ce1074af6cd084f650417b` |
| `09-测试验收与发布门禁.md` | 2784 | `057eda0f6fb0f44afd4cc4b6140800c390e9d787c8806871a55cc769680ce252` |
| `10-参考项目与资料.md` | 7086 | `85cd8ccf48b1ac8b2d441ba1a3d50028f7f32ea8c4c067a6d7c4fa6b746dcde2` |
| `11-前期材料与环境准备.md` | 10992 | `bfcb9bd5176ed99a705d1efdd84c31ff2c0d2940f5a6813578b889e3d2bb3ab6` |
| `12-审查记录与修订说明.md` | 8582 | `b17272e71f86fc2d6f411579b0c66d86151f6fcd205bde8c413334719a188bf4` |
| `README.md` | 3308 | `ec44a749f11df1a331e92013c48825e6aad98b89dcb1287a7a783df781933349` |
| `VPSKit-完整方案-v0.3-20260720.md` | 82739 | `c153341fed0dbe42540230de411d26cb7c2bfea08efd912ae622b01ea826e38c` |

## 包内白名单

本包仅包含本方案目录内的 Markdown 文档和本清单；不包含 VPS 密码、Cloudflare Token、私钥、客户端订阅链接或任何其他凭据。
