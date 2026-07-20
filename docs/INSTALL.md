# 安装与首次使用

本文描述首个正式版本 `v0.1.0` 的最终用户流程。该版本已经公开，下面的固定链接可直接下载；不要改用 `main` 分支脚本。

## 1. 前置条件

首个正式Bootstrap只允许：

- Debian 13 amd64；
- root或可执行 `sudo` 的SSH账户；
- 可访问GitHub Release、目标REALITY站点、ACME CA和Cloudflare API；
- REALITY使用的TCP端口与Hysteria2使用的UDP端口未被占用；
- 云安全组已分别放行对应的TCP和UDP端口。

balanced双协议安装还需要：

- 一个已托管到Cloudflare的域名；
- 指向VPS的DNS记录，申请证书期间保持“仅DNS”；
- 权限限制为目标Zone的Cloudflare API Token，至少具有Zone读取和DNS编辑权限；
- ACME账户邮箱。

API Token只在安装向导中隐藏输入，通过进程环境交给固定版本lego，不应写进命令、仓库或聊天记录。

## 2. 在线安装

在VPS的Bash中执行：

```bash
curl --fail --location --proto '=https' --tlsv1.2 \
  --output install.sh \
  'https://github.com/filence/vpskit/releases/download/v0.1.0/install.sh'
sudo bash install.sh
```

不采用 `curl | bash`。下载到本地的 `install.sh` 已固定仓库、版本和归档SHA-256；它只下载同一Release的归档，并依次执行：

```text
平台检查
→ 归档SHA-256
→ 路径与链接安全检查
→ 内置公钥验证签名清单
→ 中文安装向导
→ 安装前最终确认
→ 事务化部署
→ doctor
→ 客户端ZIP导出
```

向导支持balanced双协议和Reality-only。balanced可自动申请证书，也可导入已有证书作为首次安装证书；两种情况都要配置Cloudflare DNS-01，以便后续自动续期。

安装器拒绝覆盖已有 `/var/lib/vpskit/state.json`。已有安装必须使用签名包执行 `vpskit update`，不能再次运行初始安装。

## 3. 离线归档与只验证

已下载同一版本归档时：

```bash
sudo bash install.sh \
  --archive /绝对路径/v0.1.0-linux-amd64.tar.gz
```

只验证平台、归档SHA-256和签名包，不修改系统：

```bash
sudo bash install.sh \
  --archive /绝对路径/v0.1.0-linux-amd64.tar.gz \
  --verify-only
```

## 4. 安装后的管理

进入中文菜单：

```bash
sudo vpskit menu
```

菜单提供状态、完整诊断、客户端ZIP、二维码、REALITY扫描/更换、备份、证书状态/续期和中断恢复。更换目标、续期和恢复使用不同的大写确认口令，输入不完全匹配时不会执行变更。

高级或自动化场景仍可直接使用CLI：

```bash
sudo vpskit status
sudo vpskit doctor
sudo vpskit reality scan
sudo vpskit export --format bundle
```

## 5. 下载并导入客户端配置

安装结束或执行 `sudo vpskit export --format bundle` 后，终端会返回一个权限为 `0600` 的ZIP绝对路径。回到用户电脑，通过SCP/SFTP下载；不要在聊天、公开网盘或工单中传递该ZIP。

Windows PowerShell示例：

```powershell
scp -P <SSH端口> <SSH用户>@<VPS地址>:/返回的绝对路径/vpskit-client-r0001-时间.zip .
```

- Clash Verge：导入ZIP中的 `mihomo.yaml`；
- Hiddify：优先导入 `share-links.txt` 中对应协议的分享链接；
- sing-box：按协议使用 `sing-box-reality.json` 或 `sing-box-hysteria2.json`。

导入后分别测试REALITY与Hysteria2。确认客户端可用后，删除VPS上的临时ZIP和电脑上不再需要的副本。

## 6. 配置更新

静态YAML、JSON、分享链接和二维码不会自动刷新。修改端口、启停/删除实例或更换REALITY目标后，VPSKit会增加 `config_revision` 并重新生成导出；此时必须重新执行：

```bash
sudo vpskit export --format bundle
```

下载并重新导入新ZIP。更换REALITY目标前会自动扫描和端到端验证、创建回滚备份；客户端验证新修订前不要删除结果中返回的备份ID。详细流程见[客户端配置导出与更新](CLIENT_CONFIGS.md)。

## 7. 当前边界

- 不默认修改SSH、内核、BBR、本机防火墙或云安全组；
- 不提供常驻Web面板、数据库或公开订阅服务；
- Reality-only不生成Hysteria2文件；
- Debian 12、Ubuntu和arm64仍属于目标支持，不属于首个Bootstrap的已验证安装范围。
