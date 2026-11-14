package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"citadel-seeder/internal/config"
	"citadel-seeder/internal/fileutil"
	"citadel-seeder/internal/logger"
	"citadel-seeder/internal/updater"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/.output/public
var assets embed.FS

//go:embed VERSION
var embeddedVersion string

func applicationVersion() string {
	version := strings.TrimSpace(embeddedVersion)
	if version == "" {
		return "dev"
	}
	return version
}

func main() {
	if updater.RunHelperIfRequested(os.Args[1:]) {
		return
	}
	stateDirectory, err := config.StateDirectory()
	if err != nil {
		println("Startup error:", err.Error())
		return
	}
	if err := os.MkdirAll(stateDirectory, 0o700); err != nil {
		println("Startup error:", err.Error())
		return
	}
	releaseInstance, err := fileutil.TryLockFile(filepath.Join(stateDirectory, "application.lock"))
	if err != nil {
		println("Citadel Sentry state is unavailable or already owned by another instance:", err.Error())
		return
	}
	defer releaseInstance()
	logPath, logErr := logger.Init(applicationVersion())
	if logErr != nil {
		println("Failed to initialize logger:", logErr.Error())
	}
	defer logger.Close()

	defer func() {
		if r := recover(); r != nil {
			logger.Error("FATAL PANIC occurred", "panic", r, "stack", string(debug.Stack()))
			_ = logger.Close()
			panic(r)
		}
	}()

	logger.Info("Starting Citadel Sentry desktop application", "logPath", logPath)

	app, err := NewApp()
	if err != nil {
		logger.Error("Startup error in NewApp", "error", err)
		println("Startup error:", err.Error())
		return
	}
	defer app.shutdown(context.Background())

	err = wails.Run(&options.App{
		Title:            "Citadel Sentry",
		Width:            1240,
		Height:           800,
		MinWidth:         980,
		MinHeight:        640,
		Frameless:        false,
		BackgroundColour: &options.RGBA{R: 4, G: 5, B: 9, A: 255}, // Citadel pitch black #040509
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []any{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "citadel-sentry-desktop",
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		logger.Error("Wails runtime terminated with error", "error", err)
		println("Error:", err.Error())
	} else {
		logger.Info("Citadel Sentry desktop application exited cleanly")
	}
}
