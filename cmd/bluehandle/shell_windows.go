package main

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/bluehandle-open/bluehandle-go/internal/config"
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
	miOpen, miSettings, miQuit                 *application.MenuItem
	trayMu                                     sync.Mutex

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
	sh.RefreshLanguage()
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
	labels := labelsFor(sh.sess.Config().Language)
	sh.menu = sh.app.NewMenu()
	sh.miStatus = sh.menu.Add(labels.disconnected).SetEnabled(false)
	sh.menu.AddSeparator()
	sh.miOpen = sh.menu.Add(labels.open).OnClick(func(*application.Context) { sh.showWindow() })
	sh.miPause = sh.menu.Add(labels.pause).OnClick(func(*application.Context) {
		_ = sh.sess.SetPaused(!sh.sess.Status().Paused)
	})
	sh.miConnect = sh.menu.Add(labels.connect).OnClick(func(*application.Context) {
		if addr := sh.sess.Config().LastAddr(); addr != "" {
			go sh.sess.Connect(addr)
		} else {
			sh.showWindow()
		}
	})
	sh.miDisconnect = sh.menu.Add(labels.disconnect).OnClick(func(*application.Context) { _ = sh.sess.Disconnect() })
	sh.menu.AddSeparator()
	sh.miSettings = sh.menu.Add(labels.settings).OnClick(func(*application.Context) {
		sh.showWindow()
		sh.app.Event.Emit(gui.EventNavigate, "settings")
	})
	sh.miQuit = sh.menu.Add(labels.quit).OnClick(func(*application.Context) { sh.app.Quit() })

	sh.tray = sh.app.SystemTray.New().SetIcon(trayIdle).SetMenu(sh.menu)
	sh.tray.OnClick(func() {
		if sh.window.IsVisible() {
			sh.window.Hide()
		} else {
			sh.showWindow()
		}
	})
	sh.RefreshLanguage()
}

// RefreshLanguage updates native labels and connection state together.
func (sh *shell) RefreshLanguage() {
	sh.trayMu.Lock()
	defer sh.trayMu.Unlock()
	if sh.tray == nil {
		return
	}
	st := sh.sess.Status()
	cfg := sh.sess.Config()
	labels := labelsFor(cfg.Language)
	sh.window.SetTitle(labels.app)
	sh.miOpen.SetLabel(labels.open)
	sh.miConnect.SetLabel(labels.connect)
	sh.miDisconnect.SetLabel(labels.disconnect)
	sh.miSettings.SetLabel(labels.settings)
	sh.miQuit.SetLabel(labels.quit)
	connected := st.State == session.StateConnected
	icon, label := trayIdle, labels.disconnected
	switch {
	case connected && st.Paused:
		icon, label = trayPaused, labels.paused+" · "+host(st.Addr)
	case connected:
		icon, label = trayConnected, labels.connected+" · "+host(st.Addr)
	case st.State == session.StateConnecting || st.Retrying:
		label = labels.connecting + " " + host(st.Addr)
	}
	sh.tray.SetIcon(icon)
	sh.tray.SetTooltip(labels.app + " · " + label)
	sh.miStatus.SetLabel(label)
	if st.Paused {
		sh.miPause.SetLabel(labels.resume)
	} else {
		sh.miPause.SetLabel(labels.pause)
	}
	sh.miPause.SetHidden(!connected)
	sh.miDisconnect.SetHidden(!connected)
	sh.miConnect.SetHidden(connected || cfg.LastAddr() == "")
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

type trayLabels struct {
	app, disconnected, paused, connected, connecting         string
	open, pause, resume, connect, disconnect, settings, quit string
}

func labelsFor(language string) trayLabels {
	if language == config.LanguageEnglish {
		return trayLabels{
			app:          "BlueHandle",
			disconnected: "Disconnected",
			paused:       "Paused",
			connected:    "Connected",
			connecting:   "Connecting to",
			open:         "Open main window",
			pause:        "Pause input",
			resume:       "Resume input",
			connect:      "Connect to last phone",
			disconnect:   "Disconnect",
			settings:     "Settings",
			quit:         "Quit",
		}
	}
	return trayLabels{
		app:          "蓝色手柄",
		disconnected: "未连接",
		paused:       "已暂停",
		connected:    "已连接",
		connecting:   "正在连接",
		open:         "打开主窗口",
		pause:        "暂停接收",
		resume:       "继续接收",
		connect:      "连接上次的手机",
		disconnect:   "断开",
		settings:     "设置",
		quit:         "退出",
	}
}
