# VPSKit 本地源码快照清单

> 标题：VPSKit 本地源码快照清单
>
> 生成时间：2026-07-17 11:58
>
> 生成者：Codex
>
> 版本：v1.1
>
> 用途：记录方案审核使用的本地参考仓库、固定版本、提交和许可证边界

## 1. 使用边界

`source-references/repos/` 是开发机审查快照：

- 不进入 VPSKit 方案 ZIP；
- 不部署到 VPS；
- 不作为运行时依赖；
- 不代表允许复制其中代码；
- 更新快照时必须同步修改本清单和 `05-参考项目与社区调研.md`。

## 2. 已克隆仓库

| 本地目录 | 上游 | ref | HEAD commit | 许可证/边界 |
| --- | --- | --- | --- | --- |
| `repos/sing-box` | https://github.com/SagerNet/sing-box | `v1.13.14` | `25a600db24f7680ad9806ce5427bd0ab8afe1114` | GPL-3.0-or-later + 名称限制；只作能力/行为参考 |
| `repos/xray-core` | https://github.com/XTLS/Xray-core | `v26.3.27` | `d2758a023cd7f4174a5a5fa4ff66e487d4342ba0` | MPL-2.0；复用需遵守文件级义务 |
| `repos/xray-install` | https://github.com/XTLS/Xray-install | `main` | `e741a4f56d368afbb9e5be3361b40c4552d3710d` | GPL-3.0；只作安装流程参考 |
| `repos/mihomo` | https://github.com/MetaCubeX/mihomo | `v1.19.28` | `cbd11db1e13a75d8e680e0fe7742c95be4cba2be` | GPL-3.0；只作客户端字段/转换测试参考 |
| `repos/lego` | https://github.com/go-acme/lego | `v5.2.2` | `3d5a6695e027d625bd34334d516d77f578d43f11` | MIT；可集成发布二进制，保留依赖通知 |
| `repos/linux-ssh-init-sh` | https://github.com/247like/linux-ssh-init-sh | `main` | `99c0fde79b25a0fadce51d6d979f80ee2d135ef6` | MIT；优先复用思想而非大段 Shell |
| `repos/v2ray-233boy` | https://github.com/233boy/v2ray | `master` | `707ecf7601ff49f91c2d12dd22b98e8f89588d1c` | GPL-3.0；只作 CLI/UX 参考 |
| `repos/ssh_tool-eooce` | https://github.com/eooce/ssh_tool | `main` | `0b634c2aa7437cb3d4fd2fb0550f2c8573b499bc` | 根目录未发现 LICENSE；不得复制代码 |

补充说明：

- sing-box 上游默认 HEAD 为 `testing`，本快照显式使用非预发布 `v1.13.14`；
- Mihomo 当前默认 `main` 是另一个 Python 项目；本快照显式使用代理内核 Release `v1.19.28`；
- `repos/mihomo` 额外保存了 `origin/main` 单提交引用，用于证明分支差异；工作树仍停留在 `v1.19.28`；
- lego `v5.2.2` 是注释标签，表中 HEAD 是标签解析后的源码 commit。

## 3. 仅远程 HEAD 对比的仓库

以下仓库没有完整克隆，只在 2026-07-17 记录了默认分支与 HEAD：

| 上游 | 默认分支 | HEAD |
| --- | --- | --- |
| https://github.com/eooce/scripts | `master` | `c55ab47a76b4703400e2584cfd0860f7d98ea787` |
| https://github.com/fscarmen/sing-box | `main` | `4f29ea5c92707716fe5f0dfcccac12c5b5d63407` |
| https://github.com/mack-a/v2ray-agent | `master` | `85a2e321888d883744fc8fdba4fe1092ac096a47` |
| https://github.com/yonggekkk/sing-box-yg | `main` | `10300a93e69aa6ec3cac3014306f0f03af8c43fe` |
| https://github.com/RayWangQvQ/sing-box-installer | `main` | `d52d4c462f667f6ea004b1b1d78134811fd9580a` |

## 4. 更新规则

更新任一快照前：

1. 读取目标 Release Notes、默认分支和许可证；
2. 对正式核心优先选择非预发布 Release tag，不直接跟随默认分支；
3. 记录 tag object 与解析后 commit 的差别；
4. 更新 commit 后重新执行相关源码检索和文档审查；
5. 不对参考仓库创建本地修改；
6. 不把任何凭据写进 remote URL、配置或命令输出。
