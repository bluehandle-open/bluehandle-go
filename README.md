# bluehandle-go

蓝色手柄电脑端的 Go 重写版。用手机当电脑的手柄、PPT 翻页器或播放器遥控器，手机端见 [bluehandle](https://gitee.com/yunnysunny/bluehandle)，原 Java 版电脑端见 [bluehandle-pc](https://github.com/bluehandle-open/bluehandle-pc)。

提供两个程序：Windows 桌面版 `bluehandle.exe`（小窗口 + 托盘图标，可开机自启）和无窗口的调试工具 `bluehandle-cli.exe`。目标是在 Windows、macOS 和 Linux 上提供一个几 MB 的单文件程序，并与现有安卓 App 保持协议兼容。连接支持 Wi-Fi 与蓝牙两种方式。

## 功能

连接方面，可以填手机 IP 走 Wi-Fi，也可以从本机已配对的蓝牙设备里挑一台手机；最近连过的 5 台手机会留在列表里，开机后可自动重连上次的手机，意外断线会自动重试，随时可以暂停接收（连接保持，手机上的操作不再生效）。

输入注入目前只有 Windows 真正落地，用系统的 `SendInput`，不需要 cgo：按键、组合键、长按、鼠标左键 / 右键 / 中键、Unicode 文字（不受输入法影响）、回车、锁屏都支持。Windows 不允许普通程序向管理员权限的窗口发送输入，所以窗口会在非提权时提示「以管理员身份重新启动」，同意后原地重启继续工作。

窗口本身是 420×640 的固定小窗，关掉默认留在托盘，托盘图标区分未连接 / 已连接 / 已暂停，菜单里能暂停、断开、连接上次的手机、进设置、退出。只允许一个实例，第二次启动只会把已有窗口唤到前台。

## 目录结构

与 Java 版的包对应关系如下，方便对照迁移。

| Go 包 | 职责 | 对应 Java |
|---|---|---|
| `cmd/bluehandle` | 桌面程序入口：Wails 应用、窗口、托盘 | `BlueHandle.main`、`ui.*` |
| `cmd/bluehandle/frontend` | 前端（TypeScript + Vite），构建产物嵌进 exe | 无 |
| `cmd/bluehandle-cli` | 无窗口的调试工具 | 无 |
| `internal/consts` | 协议常量 | `IBluetoothConst` |
| `internal/protocol` | 帧编解码、按键与文字消息解析 | `communication.message.*` |
| `internal/keymap` | 手柄 / PPT / 播放器三套按键表 | `KeySetting` |
| `internal/input` | 把按键、文字、锁屏注入本机 | `KeyProcessUtil` |
| `internal/client` | TCP / RFCOMM 连接、读循环、状态机 | `WifiSocket`、`ClientThread` |
| `internal/bluetooth` | 蓝牙 RFCOMM 连接、已配对设备列表（Windows） | 无 |
| `internal/session` | 连接状态、自动重连、暂停、活动日志、计数 | 无 |
| `internal/config` | 设置与最近手机，存成 JSON | 无 |
| `internal/platform` | 提权、本机 IP 选择、蓝牙设置页、启动参数 | 无 |
| `internal/gui` | 绑定给前端的方法与事件 | 无 |
| `internal/discovery` | 占位包，局域网自动发现手机（待实现） | 无 |
| `docs/protocol.md` | 协议文档 | 无 |
| `site/` | Cloudflare Pages 下载站 | 无 |
| `assets/` | 图标、旧版更新日志 | `resources/` |

## 构建与测试

需要 Go 1.26（Wails v3 的要求），构建桌面版还需要 Node 22。

```sh
make test          # go test ./...
make vet           # go vet ./...
make cli           # bin/bluehandle-cli，无窗口，任意平台都能编
make gui           # bin/bluehandle.exe，先构建前端再嵌入
```

改过 `internal/gui` 里导出的方法或类型后跑 `make bindings` 重新生成前端绑定；改过图标、版本信息或 manifest 后跑 `make syso` 重新生成 `rsrc_windows_amd64.syso`。这两步需要 `wails3` 命令，日常构建不需要。

桌面版只支持 Windows，其他平台的 `cmd/bluehandle` 只会提示改用命令行版。输入注入和蓝牙也只在 Windows 上实现，其他平台自动退回打印模式。

## 命令行版

命令行程序连上手机后，把收到的按键、文字和锁屏请求在本机执行，用于排查前端不方便暴露的问题。

```sh
bin/bluehandle-cli -ip 192.168.1.5 -keys handle
bin/bluehandle-cli -ip bt:AA:BB:CC:DD:EE:FF
bin/bluehandle-cli -list-bt
```

- `-ip` 必填，填手机 IP 走 Wi-Fi，填 `bt:AA:BB:CC:DD:EE:FF` 走蓝牙。
- `-keys` 可选，取值 `handle`、`ppt`、`player`，连接后把对应按键表推给手机。
- `-list-bt` 列出本机已配对的蓝牙设备后退出。
- `-dry-run` 只打印收到的内容，不真的模拟键盘。
- 按 Ctrl+C 退出，退出前会通知手机端；连接断开时会自动松开所有仍按着的键。

## 配置

设置存在 `<用户配置目录>/bluehandle/config.json`，Windows 上是 `%AppData%\bluehandle\config.json`，删掉即恢复默认。里面除了端口、启动自动连接、断线自动重连、开机自启、关窗口到托盘、是否在活动日志里显示文字、是否弹系统通知，还有最近连过的手机和每台手机上一次的连接时长。

## 发布

推送 `v*` tag 后，`release.yml` 会编译 `bluehandle.exe` 和 `bluehandle-cli.exe` 并发布 GitHub Release，接着调用 `pages.yml`，把 `site/` 下的下载页连同这两个文件部署到 Cloudflare Pages（默认项目名 `bluehandle-go`，即 `bluehandle-go.pages.dev`）。下载地址固定为 `/download/bluehandle.exe`，版本、大小与 SHA-256 在 `/download/latest.json` 里。

首次使用前在仓库设置里添加：

- Secrets：`CLOUDFLARE_API_TOKEN`（权限 Account › Cloudflare Pages › Edit）、`CLOUDFLARE_ACCOUNT_ID`
- 可选变量：`CF_PAGES_PROJECT`，改用别的 Pages 项目名

Pages 项目不存在时会自动创建（直接上传类型）。也可以在 Actions 里手动运行 `pages` 重新部署最新版或指定 tag；本地预览可运行 `bash site/build.sh v1.0.0`（需要已登录的 `gh`、`sha256sum`、`stat`），输出在 `public/`。

### Microsoft Store

`release.yml` 同时会打一个未签名的 `bluehandle-<版本>-x64.msix`，附在 GitHub Release 和 Actions artifact 里，下载后到 Partner Center 手动上传提交，签名由商店完成。打包脚本是 `cmd/bluehandle/build/msix/pack.ps1`，清单模板是同目录的 `AppxManifest.xml`，本地可运行 `make msix VERSION=1.0.1`（需要 Windows SDK 里的 `makeappx.exe`）。

包标识默认就是商店产品的值（`whyun.bluehandle`、`CN=48325D51-0D86-4D4C-811F-9CFEA8D3F947`、发布者显示名称 `whyun`），来自 Partner Center「产品标识」页。需要换成别的值时，可以用仓库变量 `MSIX_IDENTITY_NAME`、`MSIX_PUBLISHER`、`MSIX_PUBLISHER_DISPLAY_NAME` 覆盖。

改过图标后运行 `python -I cmd/bluehandle/build/make_icons.py <目录>`，把输出里的 `msix/Assets` 拷回 `cmd/bluehandle/build/msix/Assets`。

## 路线图

1. **协议层与命令行版**（已完成）：`protocol`、`keymap`、`client` 及其测试。
2. **Windows 输入注入**（已完成）：`SendInput` 实现按键、鼠标键、Unicode 文字与锁屏。
3. **桌面界面**（已完成）：Wails v3 + TypeScript 前端，连接卡片、活动日志、设置页、托盘菜单。
4. **打包与 CI**（已完成）：Windows 构建产物、GitHub Release 与 Cloudflare Pages 下载站。
5. **蓝牙连接**（已完成）：Windows 上通过 RFCOMM 连已配对的手机。
6. **待办**：macOS 与 Linux 的桌面端和输入注入；局域网自动发现手机（需要安卓端同步修改）。

## 许可证

MIT，见 [LICENSE](LICENSE)。
