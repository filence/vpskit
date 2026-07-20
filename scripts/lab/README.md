# VPSKit lab脚本

本目录只保存开发和实机回归脚本，不是最终用户安装入口。脚本中的版本号、临时路径和预期摘要对应特定实验批次，不应直接用于新的生产VPS。

## 环境专属变量

为避免把个人域名和VPS地址提交到公开仓库，需要相关环境的脚本统一读取：

```bash
export VPSKIT_LAB_DOMAIN='node.example.com'
export VPSKIT_LAB_ZONE='example.com'
export VPSKIT_LAB_IPV4='203.0.113.10'
```

- `VPSKIT_LAB_DOMAIN`：DNS-only节点完整域名；
- `VPSKIT_LAB_ZONE`：Cloudflare Zone名称；
- `VPSKIT_LAB_IPV4`：该实验VPS的预期公网IPv4。

缺少必要变量时脚本必须立即停止，不能回退到某个开发者的默认值。

Windows本地通过 `Invoke-VPSKitSsh.ps1` 执行远程脚本时，包装器会从被 `.gitignore` 排除的 `前期环境须知.md` 读取实验域名和IPv4，仅在当前SSH会话中注入上述变量。密码和API Token不会写入这些环境变量、脚本或Git仓库。

## 安全约束

- 不把Root密码、API Token、ACME EAB、Cookie或客户端分享链接写入脚本；
- 实验用Token文件必须为临时文件并在脚本退出时删除；
- 清理命令只能处理脚本自己创建且已验证的精确路径；
- 历史部署脚本不能替代正式签名Release和 `install.sh`；
- 公开发布前必须运行Gitleaks与个人域名/IP残留扫描。

## 最终Bootstrap回归脚本

- `Prepare-Final-Bootstrap-Recovery.sh`：执行健康预检，创建受管备份、续期凭据副本和root-only完整恢复快照；
- `Deploy-Final-Bootstrap.sh`：仅在恢复快照已经离机校验后执行卸载、固定Bootstrap从零重装、自动诊断、双协议回环和安全客户端ZIP验证；失败时尝试从快照恢复原安装；
- `Verify-Final-Bootstrap-After-Reboot.sh`：重启后复核服务、证书、doctor、双协议回环和客户端ZIP权限。

这些脚本属于高影响实验工具，不能直接当作最终用户安装入口。执行前必须确认目标VPS、远端精确路径和离线恢复快照，客户端GUI人工验收完成前不得删除恢复材料。
