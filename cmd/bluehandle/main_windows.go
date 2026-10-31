// Command bluehandle is the desktop app: a small window and a tray icon that
// connect to the phone and replay its input. See cmd/bluehandle-cli for the
// windowless debugging tool.
package main

import (
	_ "embed"
	"log"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/bluehandle-open/bluehandle-go/internal/config"
	"github.com/bluehandle-open/bluehandle-go/internal/gui"
	"github.com/bluehandle-open/bluehandle-go/internal/input"
	"github.com/bluehandle-open/bluehandle-go/internal/platform"
	"github.com/bluehandle-open/bluehandle-go/internal/session"
)

// version is set with -ldflags "-X main.version=..." by the release build.
var version = "dev"

func main() {
	if pid := platform.WaitPID(os.Args[1:]); pid != 0 {
		// Restarted elevated: let the old instance release the lock first.
		platform.WaitForPID(pid, 10*time.Second)
	}

	cfgPath, err := config.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("config: %v; using defaults", err)
	}
	kb, err := input.NewSystem()
	if err != nil {
		log.Fatal(err)
	}

	sh := &shell{}
	sess := session.New(kb, cfg, func(c config.Config) error { return config.Save(cfgPath, c) }, sh)
	sh.sess = sess

	app := application.New(application.Options{
		Name:        "蓝色手柄",
		Description: "用手机当电脑的手柄、遥控器和 PPT 翻页器",
		Services:    []application.Service{application.NewService(gui.NewService(sess, sh, version))},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(frontendFS())},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "io.github.bluehandle-open.bluehandle",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { sh.showWindow() },
		},
		Windows:    application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		OnShutdown: func() { _ = sess.Disconnect() },
	})
	sh.app = app

	sh.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "蓝色手柄",
		Width:            420,
		Height:           640,
		DisableResize:    true,
		Frameless:        true,
		Hidden:           platform.HasFlag(os.Args[1:], platform.HiddenFlag),
		BackgroundColour: application.NewRGB(0x0B, 0x12, 0x20),
		URL:              "/",
	})
	sh.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if sess.Config().CloseToTray {
			e.Cancel()
			sh.window.Hide()
			return
		}
		app.Quit()
	})

	sh.setupTray()
	go sh.tickStats()
	sess.AutoStart()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
