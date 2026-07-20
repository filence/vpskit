# 第三方依赖与许可证

本清单覆盖当前 Go 模块和随 VPSKit 开发发布包分发的固定上游二进制。实际发布时还必须以 SBOM 和 `versions.lock` 复核，二者不一致即阻断发布。

| 名称 | 固定版本 | 来源 | 许可证 | 交付要求 |
| --- | --- | --- | --- | --- |
| github.com/skip2/go-qrcode | `v0.0.0-20200617195104-da1b6568686e` | https://github.com/skip2/go-qrcode | MIT | 随包附带 `licenses/go-qrcode.LICENSE` |
| github.com/SagerNet/sing-box | `v1.13.14` / commit `25a600db24f7680ad9806ce5427bd0ab8afe1114` | https://github.com/SagerNet/sing-box/tree/v1.13.14 | GPL-3.0-or-later | 随包附带上游 LICENSE，并在 `versions.lock` 记录对应源码和资产摘要 |
| github.com/XTLS/Xray-core | `v26.3.27` / commit `d2758a023cd7f4174a5a5fa4ff66e487d4342ba0` | https://github.com/XTLS/Xray-core/tree/v26.3.27 | MPL-2.0 | 随包附带上游 LICENSE；仅承载 REALITY 服务端并在 `versions.lock` 固定资产摘要 |
| github.com/go-acme/lego | `v5.2.2` / commit `3d5a6695e027d625bd34334d516d77f578d43f11` | https://github.com/go-acme/lego/tree/v5.2.2 | MIT | 随包附带上游 LICENSE |

工具链中的 ShellCheck、Staticcheck、Gitleaks 与 Syft 仅在开发/CI 使用，不部署到 VPS，也不静态链接进 VPSKit。它们的版本和下载摘要固定在 CI 工作流中。
