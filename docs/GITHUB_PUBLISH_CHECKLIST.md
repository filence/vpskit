# GitHub提交与首个Release清单

本清单区分“源码可以提交”和“一键安装可以公开使用”。两者不是同一个时间点。

## 1. 当前已完成

- [x] Debian 13 amd64双协议实机安装、固定Bootstrap从零重装与完整生命周期回归；
- [x] Clash Verge与Hiddify的REALITY/Hysteria2四项GUI验收；
- [x] schema 5客户端修订、安全ZIP和REALITY目标事务化更新；
- [x] 固定Release Bootstrap、离线归档和 `--verify-only`；
- [x] 中文安装向导与 `vpskit menu`；
- [x] Windows构建宿主下的显式Linux归档权限；
- [x] Go、Staticcheck、ShellCheck、PowerShell解析、双架构构建、隐私扫描和Gitleaks；
- [x] 普通CI和受保护草稿Release工作流；
- [x] README、安装手册、发布门禁、兼容矩阵、Changelog和正式方案包同步。

源码在完成下面第2节的仓库选择后即可提交GitHub。现在不应直接发布正式Release，因为生产签名信任根尚未建立。

## 2. 提交GitHub前需要用户确定

- [x] GitHub仓库为 `filence/vpskit`；
- [x] 仓库可见性为public；
- [x] 首个正式版本号采用 `v0.1.0`；
- [x] 用户已授权Codex执行本地首次commit、push和后续仓库内发布准备。

仓库名确定后，先把README中的正式安装URL替换为真实Release地址，再创建首个tag。

## 3. 生产签名环境

- [ ] 在离线或受控设备生成新的当前Ed25519密钥对与下一轮换公钥；
- [ ] 不使用 `.build/keys` 中的实验密钥；
- [ ] 在GitHub建立 `production-release` Environment；
- [ ] 配置需要的reviewer、禁止自批（若账户计划支持）、限制版本tag；
- [ ] 写入 `VPSKIT_RELEASE_PRIVATE_KEY`；
- [ ] 写入 `VPSKIT_RELEASE_PUBLIC_KEY`；
- [ ] 写入 `VPSKIT_RELEASE_NEXT_PUBLIC_KEY`；
- [ ] 确认三个值没有进入仓库、普通Actions变量、日志或Release资产。

## 4. 首次提交与CI

- [x] 创建空GitHub仓库，不自动生成会与本地冲突的README/License；
- [ ] 添加远端；
- [ ] 复核首个commit候选清单；
- [ ] commit并push `main`；
- [ ] 等待 `.github/workflows/ci.yml` 全部通过；
- [ ] 修复任何只在GitHub runner出现的问题；
- [ ] 确认分支保护策略。

## 5. 首个草稿Release

- [ ] 确认 `docs/releases/v0.1.0.md` 与最终仓库地址一致；
- [ ] 在CI通过的commit创建并push `v0.1.0` tag；
- [ ] 手工运行 `Protected draft release`，输入 `v0.1.0`；
- [ ] 审批 `production-release` Environment；
- [ ] 工作流必须生成签名归档、`install.sh`、checksums、manifest、signature、versions.lock、SBOM和attestation；
- [ ] 工作流只创建Draft，不自动公开。

## 6. 草稿人工复核

- [ ] Release tag与构建commit一致；
- [ ] `checksums.txt`覆盖所有公开资产；
- [ ] `install.sh`内嵌仓库、版本和归档SHA-256正确；
- [ ] `gh attestation verify`验证归档、安装器和checksums；
- [ ] 在一台干净Debian 13 amd64 VPS先运行 `--verify-only`；
- [ ] 再执行一次正式一键安装、doctor、客户端ZIP下载与双协议连接；
- [ ] Release Notes中的迁移、回滚、已知限制和上游版本准确；
- [ ] 最后由用户明确确认Publish。

只有第6节全部通过并发布后，README中的一键安装命令才可视为面向普通用户可用。
