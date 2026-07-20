# VPSKit v0.1.0第一阶段实机验收报告

> 标题：VPSKit v0.1.0第一阶段实机验收报告
>
> 生成时间：2026-07-20 16:49
>
> 生成者：Codex
>
> 版本：v1.0
>
> 用途：记录公开v0.1.0普通用户实机回归、证据边界和第一阶段收尾决定

## 1. 验收结论

2026-07-20，VPSKit公开[`v0.1.0`](https://github.com/filence/vpskit/releases/tag/v0.1.0)已完成一台重装后VPS上的普通用户端到端回归。固定版本安装、客户端配置交付和两个协议的实际使用均通过，第一阶段在本文限定范围内结论为 `PASS`。

该结论是单一系统与架构上的正式实机证据，不表示所有目标平台已经验证。

## 2. 验收环境与流程

- 服务端：Debian 13、amd64、systemd；
- 安装来源：公开 `v0.1.0` Release固定安装器；
- 部署Profile：`balanced`；
- 服务端协议：Xray REALITY与sing-box Hysteria2；
- 客户端：Clash Verge、Hiddify；
- 操作角色：用户按照公开教程自行执行，不依赖开发环境中的实验脚本。

实际流程：

1. 在重装后的VPS完成系统和网络前置检查；
2. 从固定Release下载安装器并完成一键部署；
3. 执行安装后的自动诊断并生成客户端ZIP；
4. 通过SCP把客户端ZIP下载到用户电脑；
5. 将配置导入Clash Verge与Hiddify；
6. 分别验证REALITY与Hysteria2实际可用。

## 3. 验收证据

| 验收项 | 结果 | 证据来源 |
| --- | --- | --- |
| 公开固定版本安装 | PASS | 用户于2026-07-20完成实机流程 |
| 安装后诊断与服务启动 | PASS | 安装流程和用户实机确认 |
| 客户端ZIP生成与SCP下载 | PASS | 用户确认文件已下载到本机 |
| Clash Verge导入 | PASS | 用户人工GUI测试 |
| Hiddify导入 | PASS | 用户人工GUI测试 |
| REALITY实际连接 | PASS | 用户人工公网测试 |
| Hysteria2实际连接 | PASS | 用户人工公网测试 |
| 发布资产完整性 | PASS | checksums、Ed25519清单、SBOM、Linux权限和GitHub attestation已复核 |
| 主分支自动检查 | PASS | Go、配置生成、Shell/Secrets、双架构构建和三个CLI smoke任务通过 |

实验阶段的生命周期、回滚、重启持久化和客户端解析证据见[兼容性与证据等级](COMPATIBILITY.md)；首发发布门见[GitHub提交与首个Release清单](GITHUB_PUBLISH_CHECKLIST.md)。

## 4. 未外推的范围

以下项目不阻塞本次第一阶段收尾，但仍不能标记为“已验证”：

- Debian 12与Ubuntu 24.04的systemd云实机；
- amd64以外的arm64云实机；
- 1C1G环境的长时间稳定运行；
- 10 GB磁盘在长期日志、备份和更新中的增长表现；
- 不同VPS厂商云安全组、防火墙和网络质量组合。

后续扩展或兼容性声明必须单独取得对应CI或实机证据。

## 5. 安全与运维留项

- 客户端ZIP包含节点凭据；导入完成后应删除VPS上的临时ZIP，并仅在受控位置保留必要副本；
- Cloudflare、ACME等外部账户凭据的轮换和生命周期由部署者管理，VPSKit不会擅自修改外部账户；
- VPSKit不会默认修改SSH、内核、BBR、系统防火墙或云安全组；部署者仍需按安装教程完成网络放行；
- 本报告和公开文档不记录真实VPS IP、域名、邮箱或任何实际凭据。

## 6. 方案文档归档策略

方案拆分文档和单文件同步版继续作为可审阅的Markdown保留在源码仓库。重复的方案ZIP不纳入Git跟踪，原因是二进制文件无法有效审阅差异、会重复占用仓库历史，并容易与源文档失去同步。

需要离线文档包时，可在项目根目录按需生成并校验：

```powershell
pwsh -File .\scripts\build-plan-package.ps1
pwsh -File .\scripts\verify-plan-package.ps1
```

生成的ZIP仅作为本地传递物；产品Release继续只提供安装器、签名归档、摘要、清单、版本锁和SBOM等运行所需资产。

## 7. 收尾决定

- `v0.1.0`作为第一阶段验收基线冻结，不移动或重建现有tag；
- 第一阶段状态：`PASS / CLOSED`；
- 新功能、额外平台实机验证和长期稳定性观察进入后续版本；
- 若后续修改端口、凭据或REALITY目标，必须重新导出并导入客户端配置。
