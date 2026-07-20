# 客户端配置导出与更新

本文说明VPSKit首发版本的安全导出、电脑下载、客户端导入、REALITY目标变更和回滚流程。

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

核心版本更新、服务重启、同域名证书续期不会改变客户端连接参数，因此不递增客户端配置修订号。

`vpskit status` 和 `vpskit export` 都会返回当前 `config_revision`。静态YAML、JSON、分享链接和二维码不会自行更新；修订号变化后必须重新导出并导入。

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

## 6. 首发订阅边界

首个正式版本默认不提供持久在线订阅服务，原因是REALITY已经占用TCP/443，新增HTTPS订阅通常需要独立端口、访问令牌、证书、限流、日志脱敏和令牌轮换。

后续订阅模块必须满足：

- HTTPS；
- 高强度随机Token；
- 可吊销和轮换；
- `Cache-Control: no-store`；
- 无目录列表；
- 访问日志不记录完整Token；
- 不依赖公开第三方订阅转换服务。

在该模块进入已验证状态前，以SSH/SFTP配置包和终端二维码作为稳定交付路径。
