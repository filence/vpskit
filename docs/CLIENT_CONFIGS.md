# 客户端配置导出与更新

本文说明 VPSKit v0.2.0 的安全导出、电脑下载、客户端导入、自动订阅、规则更新、REALITY 目标变更与回滚流程。

## 1. 生成安全配置包

在VPS执行：

```bash
sudo vpskit export --format bundle
```

不指定 `--output-dir` 时，VPSKit把ZIP写入发起 `sudo` 的SSH用户主目录，并把文件所有者设置为该用户。ZIP权限为 `0600`，已有同名文件不会被覆盖。

如需显式指定目录，该目录必须是已经存在的绝对路径且不能是符号链接：

```bash
sudo vpskit export --format bundle --output-dir /绝对路径/目录
```

显式目录模式保留root所有权，适合管理员受控归档；普通用户下载应使用默认模式。

命令结果包含：

- ZIP绝对路径；
- 配置修订号；
- 当前Profile；
- 包内受管配置文件数量；
- 包含凭据与下载后删除提示。

ZIP内的 `manifest.json` 记录每个配置文件的SHA-256、大小、生成时间、Profile和配置修订号；`README.txt` 提供离线导入说明。

## 2. 下载到Windows电脑

PowerShell 7示例：

```powershell
$remoteFile = 'SSH用户@VPS地址:/home/SSH用户/vpskit-client-r0001-YYYYMMDD-HHMMSS.zip'
$downloadDirectory = Join-Path $env:USERPROFILE 'Downloads'
$nativeArgs = @('-P', '22', $remoteFile, $downloadDirectory)

& 'scp.exe' @nativeArgs
if ($LASTEXITCODE -ne 0) {
    throw "客户端配置下载失败，退出代码：$LASTEXITCODE"
}
```

SSH不是22端口时，替换数组中的端口值。也可以通过WinSCP/SFTP下载，不需要额外开放Web端口。

导入并验证成功后，删除VPS上的临时ZIP：

```bash
rm -- /home/SSH用户/vpskit-client-r0001-YYYYMMDD-HHMMSS.zip
```

删除命令中的路径必须替换为导出结果返回的准确文件路径。

## 3. 客户端导入

### Clash Verge Rev

1. 解压ZIP；
2. 在配置页面选择导入本地配置；
3. 选择 `mihomo.yaml`；
4. 启用新配置；
5. 分别测试REALITY和Hysteria2。

### Hiddify

从 `share-links.txt` 复制 `vless://` 或 `hysteria2://` 链接，也可以在VPS执行以下命令后扫描终端二维码：

```bash
sudo vpskit export --format qr
```

二维码输出不回显原始链接，但二维码本身仍是凭据，不能公开截图。

### sing-box

按协议导入 `sing-box-reality.json` 或 `sing-box-hysteria2.json`。不同图形客户端的整份JSON导入入口可能不同，因此配置包同时提供标准分享链接。

## 4. 配置修订号

初始安装的客户端配置修订号为 `r0001`。以下实例变更会递增修订号并重新生成所有相关客户端产物：

- 启用、禁用或删除协议实例；
- 修改REALITY TCP端口；
- 修改REALITY目标和 `serverName`；
- 修改Hysteria2 UDP端口。
- 启用或关闭 Hysteria2 Salamander；
- 启用或关闭已经准备完成的 Hysteria2 端口跳跃。

核心版本更新、服务重启、同域名证书续期不会改变客户端连接参数，因此不递增客户端配置修订号。

`vpskit status` 和 `vpskit export` 都会返回当前 `config_revision`。静态YAML、JSON、分享链接和二维码不会自行更新；修订号变化后必须重新导出并导入。

### 4.1 v0.2.0 Cloudflare 订阅

已配置 Workers 订阅的节点会在节点、协议或客户端可见配置变化后自动发布新修订。客户端应导入该节点生成的明确 URL：

```text
https://<subscription-host>/s/<read-token>/mihomo
https://<subscription-host>/s/<read-token>/v2rayn
```

- Mihomo/Clash Verge 导入 `mihomo` URL；
- v2rayN 导入 `v2rayn` URL；
- 静态导出仍作为订阅故障时的离线回退，不具备自动更新能力；
- 订阅 URL 含读取凭据，泄露后使用 `vpskit subscription rotate-read-token` 轮换，并视需要立即吊销旧 Token。

## 5. 更换REALITY目标

先查看候选：

```bash
sudo vpskit reality scan
```

修改目标：

```bash
sudo vpskit instance modify reality \
  --reality-server-name www.amazon.com
```

正式变更前会完成：

1. 域名格式检查；
2. TLS 1.3、ALPN、X25519和证书检查；
3. 临时REALITY端到端握手；
4. 创建持久回滚备份；
5. 临时渲染并分别通过sing-box/Xray配置检查；
6. 原子写入服务端和客户端配置；
7. 重启与健康检查；
8. 失败时恢复原文件和服务状态。

成功结果中的 `changed_client_fields` 会列出 `reality.server_name`、`reality.port` 等变化，`client_update_required` 为 `true`，并返回 `previous_backup_id`。

随后重新执行：

```bash
sudo vpskit export --format bundle
```

下载并导入新修订配置。若客户端验证失败，使用命令结果中的备份ID恢复：

```bash
sudo vpskit restore BK-准确备份ID --yes
```

恢复完成后重新导出旧修订对应的客户端配置。

## 6. 订阅与规则边界

v0.2.0 使用可选的 Cloudflare Workers/KV 后端提供订阅；VPS 不持有 Cloudflare 账户级管理 Token。后端使用 HTTPS、独立的高强度读 Token、`Cache-Control: no-store`、ETag/条件 GET、Token 轮换与吊销，不提供目录列表，也不依赖公开订阅转换服务。

已发布的 ACL4SSR 和 anti-AD 规则会随 Mihomo 主订阅引用更新。规则源刷新采用完整下载、限额与哈希校验后原子激活：任何源失败时，客户端继续使用当前完整修订。

anti-AD 可能误拦截少数域名。遇到问题时先确认客户端日志，再使用精确域名白名单；这项长期误杀观察不影响 v0.2.0 已完成的部署、订阅、更新和连接验收。
