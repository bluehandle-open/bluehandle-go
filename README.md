# bluehandle-go

蓝色手柄电脑端的 Go 重写版。用手机当电脑的手柄、PPT 翻页器或播放器遥控器，手机端见 [bluehandle](https://gitee.com/yunnysunny/bluehandle)，原 Java 版电脑端见 [bluehandle-pc](https://github.com/bluehandle-open/bluehandle-pc)。

目标是在 Windows、macOS 和 Linux 上提供一个几 MB 的单文件程序，并与现有安卓 App 保持协议兼容。蓝牙方式不再支持，只保留 Wi-Fi。

## 目录结构

与 Java 版的包对应关系如下，方便对照迁移。

| Go 包 | 职责 | 对应 Java |
|---|---|---|
| `cmd/bluehandle` | 程序入口 | `BlueHandle.main` |
| `internal/consts` | 协议常量 | `IBluetoothConst` |
| `internal/protocol` | 帧编解码、按键消息解析 | `communication.message.*` |
| `internal/keymap` | 手柄 / PPT / 播放器三套按键表 | `KeySetting` |
| `internal/input` | 把按键事件注入本机 | `KeyProcessUtil` |
| `internal/client` | TCP 连接、读循环、状态机 | `WifiSocket`、`ClientThread` |
| `internal/ui` | 桌面窗口（待实现） | `ui.*` |
| `internal/discovery` | 局域网自动发现手机（待实现） | 无 |
| `docs/protocol.md` | 协议文档 | 无 |
| `assets/` | 图标、旧版更新日志 | `resources/` |

## 构建与测试

需要 Go 1.22 或更新版本。

```sh
go test ./...
go build -o bin/bluehandle ./cmd/bluehandle
```

当前阶段的命令行程序只打印收到的按键，不会真的模拟键盘：

```sh
bin/bluehandle -ip 192.168.1.5 -keys handle
```

`-keys` 可选，取值 `handle`、`ppt`、`player`，连接后把对应按键表推给手机。按 Ctrl+C 退出，退出前会通知手机端。

## 路线图

1. **协议层与命令行版**（当前）：`protocol`、`keymap`、`client` 及其测试，命令行连上手机打印按键。
2. **真实按键注入**：`input` 增加 robotgo 实现，需要 cgo；macOS 需要在“辅助功能”里授权，Linux 仅支持 X11。
3. **桌面界面**：用 Fyne 复刻 Java 版的 Wi-Fi 面板、按键设置和状态栏。
4. **打包与 CI**：三平台构建产物。
5. **局域网发现**：新增 UDP 发现，需要安卓端同步修改。

文本输入（`0x50`）与锁屏（`0x51`）的协议解析已经完成，命令行版会打印出来；真正输入文字、锁屏随第 2 阶段的 `input` 实现一起做。

## 许可证

MIT，见 [LICENSE](LICENSE)。
