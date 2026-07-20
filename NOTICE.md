# VPSKit Notices

VPSKit 自身以 MIT License 发布。发布包还包含或静态链接以下第三方组件；精确版本、来源 tag/commit、上游资产摘要和用途以同一发布包中的 `versions.lock`、`release-manifest.json` 与 `THIRD_PARTY_LICENSES.md` 为准。

| 组件 | 集成方式 | 许可证 | 用途 |
| --- | --- | --- | --- |
| SagerNet/sing-box | 原样随发布包分发的官方固定版本二进制 | GPL-3.0-or-later | Hysteria2 服务端核心、客户端导出及配置校验 |
| XTLS/Xray-core | 原样随发布包分发的官方固定版本二进制 | MPL-2.0 | REALITY 服务端核心及配置校验 |
| go-acme/lego | 原样随发布包分发的官方固定版本二进制 | MIT | ACME DNS-01 证书申请与续期 |
| skip2/go-qrcode | 静态链接进 VPSKit | MIT | 终端二维码渲染 |

构建脚本会把 VPSKit、sing-box、Xray、lego 和 go-qrcode 的许可证文本放入发布包。VPSKit 不修改或冒充上述项目，也不向第三方在线转换服务发送节点凭据。
