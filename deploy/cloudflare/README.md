# VPSKit Cloudflare 订阅 Worker

本目录包含 v0.2.0 单 VPS 自动订阅后端。Worker 只暴露三类读取目标：

```text
/s/<read-token>/mihomo
/s/<read-token>/v2rayn
/s/<read-token>/manifest
```

发布接口使用节点级 HMAC 凭据，不接受 Cloudflare 管理 Token。Cloudflare 管理 Token 仅用于可信管理端创建 Worker、KV、Secret 和自定义域名。

安全边界：

- `NODE_PUBLISH_SECRET` 与 `INITIAL_READ_TOKEN_HASH` 必须作为 Worker Secret；
- VPS 只保存节点发布 Secret 和订阅读取 Token；
- 发布请求有时间窗、随机 nonce、HMAC、body 上限、摘要和 schema 校验；
- KV 中每个 target 的完整修订先写历史，再切换 current；
- Workers KV 是最终一致性存储，远端回读超时应标记 `DEGRADED`，不能宣称全局原子提交；
- 访问日志不得记录完整订阅路径。

## Windows 11 预发布部署

使用 PowerShell 7，先做零写入计划：

```powershell
pwsh -NoLogo -NoProfile -File scripts/cloudflare/Deploy-SubscriptionWorker.ps1 `
  -NodeId node-main `
  -ZoneName example.com `
  -Hostname sub-dev.example.com
```

`-Apply` 模式要求一个单独的 Cloudflare 管理 Token。Token 至少需要目标账户的 Workers Scripts 写入、Workers KV Storage 写入，以及目标 Zone 的读取和 Workers Routes 写入权限。可通过当前进程的 `CLOUDFLARE_API_TOKEN` 或受限的 `-ManagementTokenFile` 传入；不要写进仓库、参数或普通日志。

脚本固定使用 Wrangler `4.112.0`，创建/复用同名开发 KV、创建新 Worker、写入 Worker Secret、绑定 Custom Domain，并在健康检查通过后输出一份受限凭据 JSON。为防止无意轮换，若 Worker 或输出文件已存在，脚本会停止而不是覆盖。

官方依据：

- [Cloudflare Workers Custom Domains](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/)
- [Cloudflare KV namespace API](https://developers.cloudflare.com/api/resources/kv/subresources/namespaces/methods/create/)
- [Cloudflare API Token permissions](https://developers.cloudflare.com/fundamentals/api/reference/permissions/)
- [Wrangler install/update](https://developers.cloudflare.com/workers/wrangler/install-and-update/)

本地无依赖测试：

```bash
node --test deploy/cloudflare/worker.test.mjs
```

`wrangler.toml.example` 只提供绑定模板，不包含账号 ID、namespace ID 或秘密。
