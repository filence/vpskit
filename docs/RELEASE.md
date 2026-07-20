# 发布门禁

正式 Release 必须同时满足以下条件：

1. `go mod verify`、gofmt、vet、单元测试、Staticcheck、ShellCheck 和 Gitleaks 全部通过。
2. 固定 sing-box 版本能够解析生成的服务端/客户端配置；模板 golden tests 无非预期变化。
3. Linux amd64/arm64 构建成功，checksums、签名 `release-manifest.json`、`versions.lock` 和 SPDX SBOM 同步生成。
4. 发布包包含 `LICENSE`、`NOTICE.md`、`THIRD_PARTY_LICENSES.md` 和对应第三方许可证文本。
5. 兼容矩阵仅把有 CI/VM/云实机证据的平台标为已验证。
6. 发布说明包含迁移、回滚、已知问题和上游 tag/commit；不推荐 `curl main | bash`。
7. 发布签名私钥只从受保护的发布环境注入，不写入仓库、普通 CI 或日志。
8. 客户端 ZIP 必须验证权限、条目白名单、配置修订号、每项 SHA-256、拒绝覆盖和 Reality-only 条目裁剪。
9. REALITY 目标修改必须验证失败不写入、成功递增修订号、自动重导出、返回回滚点，并完成至少一次 GUI 客户端重新导入测试。
10. Linux归档必须由发布工具以显式 `0755/0644` 权限生成；Bootstrap必须固定仓库、版本和归档SHA-256，并通过 `--verify-only` 实机验证。
11. 中文管理菜单必须覆盖常用只读操作；REALITY更换、证书续期和中断恢复必须使用不同的精确确认口令。

## 受保护发布流程

`.github/workflows/release.yml` 只接受手工输入的现有版本tag：

1. 无密钥 `source-gate` 先执行格式、依赖、vet、测试、Staticcheck、ShellCheck和Gitleaks；
2. 通过后才进入 `production-release` Environment；
3. 从Environment secrets注入当前私钥、当前公钥和下一轮换公钥；
4. 固定下载并校验Xray、sing-box、lego和Syft；
5. 生成签名清单、版本锁、SPDX SBOM、显式Linux权限归档和固定版本 `install.sh`；
6. 使用GitHub官方 `actions/attest` 生成构建来源证明；
7. 只创建草稿Release，人工复核后才允许公开。

公开仓库为 `filence/vpskit`。`production-release` Environment已经创建，配置了 `filence` reviewer与 `v0.*` tag限制，并写入以下三个secret：

- `VPSKIT_RELEASE_PRIVATE_KEY`；
- `VPSKIT_RELEASE_PUBLIC_KEY`；
- `VPSKIT_RELEASE_NEXT_PUBLIC_KEY`。

开发机 `.build/keys` 内的实验密钥没有用于生产。当前与下一轮换密钥对在受控开发设备重新生成，两套完整密钥只保存在仓库外的ACL隔离私有目录；生产私钥不得写入仓库、普通CI、Release资产或日志。

当前工作流代码、仓库、首发版本、生产Environment、生产签名信任根、正式tag与草稿资产已经就绪。工作流本身仍只创建草稿；`v0.1.0`在资产复核后由用户明确确认公开，并以公开在线安装完成最后的普通用户实机回归。

GitHub机制依据：[Environment与审批保护](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments)、[构建来源证明](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)、[`gh release create`草稿与tag校验](https://cli.github.com/manual/gh_release_create)。
