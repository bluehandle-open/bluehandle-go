package main

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/bluehandle-open/bluehandle-go/internal/gui"
	"github.com/bluehandle-open/bluehandle-go/internal/platform"
	"github.com/bluehandle-open/bluehandle-go/internal/session"
)

var (
	//go:embed icons/tray_idle.png
	trayIdle []byte
	//go:embed icons/tray_connected.png
	trayConnected []byte
	//go:embed icons/tray_paused.png
	trayPaused []byte
)

// shell connects the session to the window, the tray and the frontend. It
// implements session.Sink and gui.Host.
type shell struct {
	app    *application.App
	window *application.WebviewWindow
	sess   *session.Session

	tray                                       *application.SystemTray
	menu                                       *application.Menu
	miStatus, miPause, miConnect, miDisconnect *application.MenuItem

	mu   sync.Mutex
	last session.Status
}

// OnStatus implements session.Sink.
func (sh *shell) OnStatus(st session.Status) {
	sh.mu.Lock()
	sh.last = st
	sh.mu.Unlock()
	if sh.app == nil {
		return
	}
	sh.app.Event.Emit(gui.EventStatus, st)
	sh.updateTray(st)
}

// OnActivity implements session.Sink.
func (sh *shell) OnActivity(a session.Activity) {
	if sh.app != nil {
		sh.app.Event.Emit(gui.EventActivity, a)
	}
}

// SetStartOnBoot implements gui.Host. The login item starts the app hidden
// in the tray.
func (sh *shell) SetStartOnBoot(on bool) error {
	if !on {
		return sh.app.Autostart.Disable()
	}
	return sh.app.Autostart.EnableWithOptions(application.AutostartOptions{
		Identifier: "bluehandle",
		Arguments:  []string{platform.HiddenFlag},
	})
}

// RestartElevated implements gui.Host.
func (sh *shell) RestartElevated() error {
	var args []string
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, platform.WaitPIDFlag) {
			args = append(args, a)
		}
	}
	if err := platform.RestartElevated(args); err != nil {
		return err
	}
	sh.app.Quit()
	return nil
}

func (sh *shell) showWindow() {
	sh.window.Show()
	sh.window.Restore()
	sh.window.Focus()
}

func (sh *shell) setupTray() {
	sh.menu = sh.app.NewMenu()
	sh.miStatus = sh.menu.Add("未连接").SetEnabled(false)
	sh.menu.AddSeparator()
	sh.menu.Add("打开主窗口").OnClick(func(*application.Context) { sh.showWindow() })
	sh.miPause = sh.menu.Add("暂停接收").OnClick(func(*application.Context) {
		_ = sh.sess.SetPaused(!sh.sess.Status().Paused)
	})
	sh.miConnect = sh.menu.Add("连接上次的手机").OnClick(func(*application.Context) {
		if addr := sh.sess.Config().LastAddr(); addr != "" {
			go sh.sess.Connect(addr)
		} else {
			sh.showWindow()
		}
	})
	sh.miDisconnect = sh.menu.Add("断开").OnClick(func(*application.Context) { _ = sh.sess.Disconnect() })
	sh.menu.AddSeparator()
	sh.menu.Add("设置").OnClick(func(*application.Context) {
		sh.showWindow()
		sh.app.Event.Emit(gui.EventNavigate, "settings")
	})
	sh.menu.Add("退出").OnClick(func(*application.Context) { sh.app.Quit() })

	sh.tray = sh.app.SystemTray.New().SetIcon(trayIdle).SetMenu(sh.menu)
	sh.tray.OnClick(func() {
		if sh.window.IsVisible() {
			sh.window.Hide()
		} else {
			sh.showWindow()
		}
	})
	sh.updateTray(sh.sess.Status())
}

func (sh *shell) updateTray(st session.Status) {
	if sh.tray == nil {
		return
	}
	connected := st.State == session.StateConnected
	icon, label := trayIdle, "未连接"
	switch {
	case connected && st.Paused:
		icon, label = trayPaused, "已暂停 · "+host(st.Addr)
	case connected:
		icon, label = trayConnected, "已连接 · "+host(st.Addr)
	case st.State == session.StateConnecting || st.Retrying:
		label = "正在连接 " + host(st.Addr)
	}
	sh.tray.SetIcon(icon)
	sh.tray.SetTooltip("蓝色手柄 · " + label)
	sh.miStatus.SetLabel(label)
	if st.Paused {
		sh.miPause.SetLabel("继续接收")
	} else {
		sh.miPause.SetLabel("暂停接收")
	}
	sh.miPause.SetHidden(!connected)
	sh.miDisconnect.SetHidden(!connected)
	sh.miConnect.SetHidden(connected || sh.sess.Config().LastAddr() == "")
	sh.menu.Update()
}

// tickStats pushes the counters once a second while connected so the
// heartbeat age stays fresh on screen.
func (sh *shell) tickStats() {
	for range time.Tick(time.Second) {
		sh.mu.Lock()
		connected := sh.last.State == session.StateConnected
		sh.mu.Unlock()
		if connected {
			sh.app.Event.Emit(gui.EventStats, sh.sess.Stats())
		}
	}
}

// host strips the default port for display.
func host(addr string) string {
	return strings.TrimSuffix(addr, ":8088")
}
