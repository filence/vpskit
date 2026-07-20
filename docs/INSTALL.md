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

### 1.1 更新全新系统

VPS服务商提供的Debian 13镜像可能早于当前安全更新。首次部署VPSKit前，建议由用户明确执行一次常规系统更新：

```bash
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y ca-certificates curl
sudo reboot
```

如果当前已经是root账户，可以去掉命令前的 `sudo`。执行 `reboot` 后SSH连接会断开；等待VPS重新启动并重新连接，再继续下面的端口放行和VPSKit安装。

这里的 `apt-get update` 只刷新软件包索引，`apt-get upgrade` 才会安装当前Debian 13的软件与安全更新。不要修改APT软件源把Debian 12直接升级到13，也不要在不了解依赖变化时改用 `full-upgrade`。首个正式Bootstrap只验证全新安装的Debian 13 amd64。

VPSKit安装器不会静默执行系统升级，因为升级可能重启SSH等服务、更新内核并要求重启，或遇到软件包锁和配置交互。系统更新始终保留为安装前的人工步骤。依据可参考[Debian系统维护与APT说明](https://www.debian.org/doc/manuals/debian-handbook/index.en.html)和[Debian安全更新FAQ](https://www.debian.org/security/faq.en.html)。

### 1.2 放行TCP/UDP端口

VPSKit默认让REALITY使用TCP/443、Hysteria2使用UDP/443。二者协议不同，可以共用数字端口443。安装前进入VPS服务商控制台，在实例关联的“安全组”“云防火墙”或“网络防火墙”中添加两条入站规则：

| 用途 | 方向 | 协议 | 目标端口 | IPv4来源 | 动作 |
| --- | --- | --- | ---: | --- | --- |
| SSH管理 | 入站 | TCP | 实际SSH端口 | 现有SSH来源范围 | 允许 |
| REALITY | 入站 | TCP | 443 | `0.0.0.0/0` | 允许 |
| Hysteria2 | 入站 | UDP | 443 | `0.0.0.0/0` | 允许 |

- 不要为了省事放行全部端口，也不要删除现有SSH入站规则；
- VPS与客户端都使用IPv6时，再分别添加来源为 `::/0` 的TCP/443和UDP/443规则；
- 如果在安装向导中改用自定义端口，安全组也必须改为放行对应协议的同一端口；
- Cloudflare中的节点DNS记录必须保持“仅DNS”，不能开启橙色云代理。

服务商安全组是第一层入站防护。对于全新、专用于VPSKit的VPS，推荐再启用UFW作为主机侧第二层防护；两层都必须放行同一端口。VPSKit使用Cloudflare DNS-01申请证书，不需要开放TCP/80。

不要同时混用UFW、firewalld和手写nftables规则。标准Debian 13通常没有启用UFW；下面的流程会先识别并放行当前SSH端口，再启用防火墙。执行期间保持当前SSH窗口和服务商网页控制台可用：

```bash
ssh_port="$(sudo /usr/sbin/sshd -T | awk '$1 == "port" {print $2; exit}')"
if ! printf '%s\n' "$ssh_port" | grep -Eq '^[0-9]+$'; then
    echo '无法识别SSH端口，停止配置UFW' >&2
else
    printf '检测到SSH端口：%s\n' "$ssh_port"
    sudo apt-get install -y ufw
    sudo ufw allow "$ssh_port"/tcp comment 'SSH'
    sudo ufw allow 443/tcp comment 'VPSKit REALITY'
    sudo ufw allow 443/udp comment 'VPSKit Hysteria2'
    sudo ufw default deny incoming
    sudo ufw default allow outgoing
    sudo ufw show added
fi
```

确认输出中的SSH、TCP/443和UDP/443规则无误后再启用：

```bash
sudo ufw enable
sudo ufw status verbose
```

不要立即关闭原SSH窗口。打开第二个终端重新连接：新连接成功后才能关闭旧窗口；如果失败，在旧窗口执行 `sudo ufw disable` 恢复访问并重新检查SSH端口。使用非默认VPSKit端口时，把命令中的443替换成安装向导中实际填写的端口。

如果系统已经由firewalld或手写nftables管理，不要再启用UFW，应在现有防火墙中添加等价规则。VPSKit不会自动修改主机防火墙或云安全组。UFW的启用、默认策略和远程管理行为可参考[Debian UFW手册](https://manpages.debian.org/trixie/ufw/ufw.8.en.html)和[Debian UFW说明](https://wiki.debian.org/Uncomplicated%20Firewall%20%28ufw%29)。

安装前可检查443是否已被其他程序占用；没有输出表示当前没有监听者：

```bash
sudo ss -lntup | grep -E ':443\b'
```

安装完成后再次运行同一命令，应同时看到TCP监听和UDP监听。Windows PowerShell可用 `Test-NetConnection <VPS-IP> -Port 443` 检查TCP外部连通性；UDP是否可用应以Hiddify或其他Hysteria2客户端的实际连接测试为准。

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
