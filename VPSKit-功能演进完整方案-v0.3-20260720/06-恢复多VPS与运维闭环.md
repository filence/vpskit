# 06｜恢复、多 VPS 与运维闭环

> 标题：VPSKit 恢复、多 VPS 与运维闭环方案
>
> 生成时间：2026-07-21 09:05
>
> 生成者：Codex
>
> 版本：v0.3-R1
>
> 用途：区分本地备份与可移植快照，定义重装恢复、多 VPS、修复、清理和诊断边界

## 1. 先区分两类恢复能力

### 1.1 当前本地 backup/restore

v0.1.0 已有受管备份与恢复，目标是同一台 VPS 上的事务回滚和配置恢复。它不是面向长期离线保存的加密灾备包。

### 1.2 未来 portable snapshot

可移植快照用于：

- VPS 重装；
- 更换系统或架构；
- 离线保存；
- 恢复节点身份、规则和订阅关系。

文档、命令和状态必须使用不同名称，避免用户把本地备份误认为已加密的离线灾备。

## 2. 两种重装模式

### 全新部署

- 生成新 UUID、Reality 密钥和 Hy2 密码；
- 重新签发证书；
- 生成新节点/订阅身份或替换旧节点；
- 吊销旧订阅 Token；
- 适合凭据可能泄露或希望完全重置。

### 恢复部署

- 恢复节点元数据和配置偏好；
- 按策略恢复或轮换协议凭据；
- 恢复规则选择；
- 重建订阅关系；
- 重新下载当前架构的固定核心二进制；
- 重新签发或校验证书；
- WARP 默认重新注册，不直接搬运设备注册状态。

## 3. 可移植加密快照

命令：

```bash
vpskit snapshot export
vpskit snapshot inspect <file>
vpskit snapshot restore <file> --plan
vpskit snapshot restore <file> --apply
```

建议结构：

```text
snapshot.age
├── encrypted payload
│   ├── state.json
│   ├── secrets/
│   ├── certificates/
│   ├── versions.lock
│   ├── ownership.json
│   ├── subscription.json
│   ├── ruleset.json
│   └── manifest.json
└── minimal public envelope
    ├── snapshot_schema
    ├── created_at
    ├── encryption_type
    └── encrypted_sha256
```

公共 envelope 不记录 IP、域名、node ID、客户端名称或服务商。

## 4. 加密与口令处理

优先顺序：

1. 用户提供的 age 公钥；
2. age 口令模式；
3. 未来可选硬件/密钥库集成。

要求：

- 固定 age 实现和版本，纳入签名资产与许可证清单；
- 口令通过 TTY 或文件描述符输入，不进入 argv、环境变量、日志或 shell history；
- 导出后立即执行完整性读回；
- 错误口令和篡改必须在任何写入前失败；
- 不支持普通 ZIP 密码或明文 tar；
- 快照文件和解密口令不得放在同一位置。

## 5. 快照内容边界

包含：

- 状态、节点元数据和实例配置；
- 协议 secrets；
- 受管证书和私钥；
- 版本锁、ownership、schema；
- 订阅 endpoint 元数据、read-token 策略和 node publisher 身份；
- 规则选择和用户自定义规则；
- 最近有效 Artifact Set 的摘要和可选加密副本；
- 恢复说明。

默认不包含：

- SSH 私钥；
- Cloudflare 管理级 Token；
- root 密码；
- 客户端 Cookie/账号会话；
- WARP 设备注册材料；
- 缓存、普通日志和可重新下载的核心二进制。

Cloudflare ACME Token 是否包含必须由用户显式选择；默认建议恢复时重新注入或轮换。

## 6. 恢复策略

```text
restore-identical
restore-and-rotate-subscription
restore-and-regenerate-protocol-secrets
restore-config-only
```

### identical

恢复协议凭据、节点身份和订阅读取地址。若旧 VPS 可能被攻破，不得选择。

### rotate-subscription

协议凭据不变，轮换读取和发布凭据。

### regenerate-protocol-secrets

恢复结构、规则和节点名，但重新生成协议凭据并要求客户端更新。

### config-only

只恢复非敏感配置偏好，不恢复密钥、证书或 Token。

## 7. 恢复预演

`--plan` 必须零写入并输出：

- snapshot/schema/VPSKit 兼容性；
- 系统、架构和磁盘条件；
- 将创建、覆盖或跳过的受管对象；
- 端口和服务冲突；
- 域名、DNS 和证书要求；
- 需要重新下载的当前架构二进制；
- 需要重新签发的证书；
- 是否保留订阅地址；
- 将轮换的凭据；
- 客户端是否必须更新；
- 上游固定版本是否仍可获取；
- 回滚点和失败策略。

跨 amd64/arm64 恢复只恢复可移植状态，绝不复用旧架构二进制。

## 8. 少量多 VPS

目标规模约 2～10 台，不建设远程 shell 控制面。

每台 VPS：

- 独立 node ID、状态和协议凭据；
- 独立节点级发布凭据；
- 独立本地备份和加密快照；
- 独立健康状态；
- 只发布自己的 Node Manifest。

订阅聚合器：

- 聚合节点和策略组；
- 标记地区、提供商和优先级；
- 保留上一有效节点修订；
- 可禁用故障节点；
- 不保存 SSH 密码；
- 不持有 root 权限；
- 不远程执行系统命令。

## 9. 节点命名与冲突处理

显示名建议：

```text
<提供商>-<国家/城市>-<序号>-<协议>
Provider-JP-Tokyo-01-Reality
Provider-JP-Tokyo-01-Hysteria2
```

稳定 identity 使用 `node_id`，显示名可变。聚合时若重名：

1. 添加用户可读序号；
2. 仍冲突时添加 node ID 短后缀；
3. 不用 IP、UUID 或 Token 解决冲突。

## 10. 聚合策略

```text
PERFORMANCE：Hy2 优先，基于用户选择，不凭单次延迟自动判定
STABLE：Reality 优先
REGION-*：按地区分组
AI：具备所选 AI 出站策略的节点
MANUAL：全部可用节点手选
```

健康信息只用于提示和可选禁用，不在首版做自动频繁切换。

## 11. doctor --fix

允许自动修复：

- 受管文件权限；
- 缺失受管目录；
- systemd daemon-reload；
- 配置有效但未启用的 VPSKit 服务；
- 可重建缓存；
- 未完成事务恢复；
- 订阅元数据与本地产物摘要不一致。

禁止自动：

- 修改 SSH；
- 清空或重写未知防火墙；
- 替换内核；
- 关闭未知服务；
- 删除唯一备份；
- 更新系统全部软件包；
- 轮换协议/订阅凭据；
- 重新签发证书；
- 将 WARP 隐私回退从 block 改为 direct。

每项 fix 必须有 plan、执行记录和反向操作。

## 12. cleanup

```bash
vpskit cleanup plan
vpskit cleanup apply --plan-id <id>
```

候选范围：

- 已结束旧事务；
- 失败 staging；
- 更新缓存；
- 过期客户端 ZIP；
- 旧规则缓存；
- 超额日志；
- 多余核心版本；
- 超过保留数量的备份和发布修订。

必须保留：

- 当前和上一成功核心版本；
- 当前状态和最近一次成功迁移前状态；
- 至少一个可恢复本地备份；
- 当前和上一客户端/规则发布；
- 尚未确认下载的最新快照；
- ownership 和审计索引。

`plan-id` 应绑定候选清单摘要，防止计划显示后目录变化导致误删。

## 13. 脱敏诊断包

```bash
vpskit support bundle
```

采用**字段 allowlist**，不是“收集所有文件后做字符串替换”。允许内容：

- 系统、内核、架构；
- 内存、磁盘、inode；
- 服务状态和受管端口；
- 核心版本；
- 配置/规则/发布摘要；
- 脱敏的结构化错误码；
- 最近事务状态。

不得包含：

- 协议 UUID/密码/私钥；
- TLS 私钥；
- Cloudflare/WARP/发布凭据；
- 完整订阅 URL；
- SSH 密钥或密码；
- 真实客户端配置；
- shell history、环境变量全集或任意目录 dump。

生成后运行二次 secret scan；发现疑似秘密时失败关闭，不生成“可能已脱敏”的包。
