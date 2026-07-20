# 兼容性与证据等级

“目标支持”表示代码和构建目标已建立；只有 CI 或实机证据通过后才可升级为“已验证”。容器 CLI 冒烟不能替代 systemd、低端口、IPv6、防火墙和重启持久化测试。

| 平台 | 架构 | 当前等级 | 证据 |
| --- | --- | --- | --- |
| Debian 13 | amd64 | lab33最终Bootstrap与GUI已验证 | 双核心 balanced clean install、TCP/UDP同数字端口、证书受管副本、实例启停、备份/恢复、自更新、卸载/重装、重启持久化、orphan scan、Reality扫描器、schema 5迁移、目标切换恢复、安全客户端ZIP、固定Bootstrap、中文菜单和公网GUI客户端均通过 |
| Debian 12 | amd64 | 目标支持 | CI 容器 CLI 冒烟已配置，systemd VM 尚未运行 |
| Ubuntu 24.04 | amd64 | 目标支持 | CI 容器 CLI 冒烟已配置，systemd VM 尚未运行 |
| Debian 12/13 | arm64 | 目标支持 | 交叉编译已配置，云实机尚未运行 |
| Ubuntu 24.04 | arm64 | 目标支持 | 交叉编译已配置，云实机尚未运行 |

## lab31 客户端证据

| 客户端 | 协议 | 结果 | 说明 |
| --- | --- | --- | --- |
| sing-box `v1.13.14` Windows amd64 | REALITY、Hysteria2 | 自动化通过 | 最终导出语法检查、握手与出口 IP 一致性均通过 |
| Mihomo `v1.19.29` Windows amd64 | Hysteria2 | 自动化通过 | 公网直连握手与出口 IP 一致性通过 |
| Mihomo `v1.19.29` Windows amd64 | REALITY | 隔离链路自动化通过 | 当前 Windows 已运行的 TUN 会干扰第二个 Mihomo 进程直连；经 SSH loopback 隔离后同一最终配置通过 |
| Xray `v26.3.27` Windows amd64 | REALITY | 隔离链路自动化通过 | 配置解析、握手与出口 IP 一致性通过 |
| Clash Verge、Hiddify | REALITY、Hysteria2 | lab31 人工通过 | 用户使用最终重装凭据分别验证两个客户端、两个协议，四项均可用 |

lab31 客户端发布门已完成。自动化隔离链路用于排除本机既有 TUN/代理对测试进程的干扰；最终结论以用户在 Clash Verge 与 Hiddify 中的真实 GUI 使用结果为准。

## lab32 客户端配置交付证据

- schema 4 安装由 lab32 程序归一化到 schema 5，初始客户端修订为1；
- 无效REALITY目标被拒绝且状态摘要不变；
- REALITY目标切换到已验证候选后恢复 `www.amazon.com`，配置修订递增至3，原实例凭据与Hysteria2客户端配置保持不变；
- 最终双协议VPS回环连接通过；
- 修订3 ZIP权限、条目白名单、清单大小与逐项SHA-256通过；
- 下载到Windows后的两份sing-box JSON和Mihomo YAML通过固定版本解析；
- Clash Verge/Hiddify 已重新导入修订3配置，REALITY、Hysteria2 四项 GUI 验收全部通过。

## lab33 发布入口与菜单证据

- Bootstrap内嵌版本、仓库和归档SHA-256，本地生成/编码/ShellCheck通过；
- VPS端 `--verify-only` 的平台、归档摘要和签名清单通过；
- Windows发布构建改为Go工具生成tar.gz，四个Linux程序在归档内均为 `0755`；
- 从修正后的归档原位更新到lab33后doctor通过；
- 中文菜单的状态、证书状态和单目标REALITY端到端扫描通过；
- REALITY更换未输入精确大写确认口令时，状态文件摘要不变；
- 正式收口前先创建受管备份与root-only恢复快照，快照下载到Windows后通过远端SHA-256和归档读回校验；
- 使用同一固定归档执行受管卸载与最终Bootstrap从零重装，schema 5、balanced、初始客户端修订1、证书受管副本和原REALITY目标均符合预期；
- 重装后doctor、证书状态、systemd服务、orphan scan、REALITY与Hysteria2回环、安全ZIP清单均通过；
- VPS重启后服务自启动、doctor和双协议回环再次通过；
- 新修订ZIP下载到Windows后通过传输SHA-256、ZIP条目白名单、清单大小和逐项SHA-256校验；
- Clash Verge与Hiddify分别使用新配置验证REALITY、Hysteria2，四项公网GUI人工验收全部通过；
- 用户明确确认后，远程11项恢复/安装临时材料和本机恢复目录已按精确路径删除；删除后doctor与三个systemd单元健康，accepted客户端配置保留。
