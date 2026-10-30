package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/cereals-af/cereals-desktop/internal/appupdate"
	"github.com/cereals-af/cereals-desktop/internal/buildinfo"
	"github.com/cereals-af/cereals-desktop/internal/cluster"
	"github.com/cereals-af/cereals-desktop/internal/configuration"
	"github.com/cereals-af/cereals-desktop/internal/customresource"
	"github.com/cereals-af/cereals-desktop/internal/desktop"
	"github.com/cereals-af/cereals-desktop/internal/filetransfer"
	"github.com/cereals-af/cereals-desktop/internal/kube"
	"github.com/cereals-af/cereals-desktop/internal/localstore"
	"github.com/cereals-af/cereals-desktop/internal/manifest"
	"github.com/cereals-af/cereals-desktop/internal/metrics"
	"github.com/cereals-af/cereals-desktop/internal/network"
	"github.com/cereals-af/cereals-desktop/internal/node"
	"github.com/cereals-af/cereals-desktop/internal/observe"
	"github.com/cereals-af/cereals-desktop/internal/overview"
	"github.com/cereals-af/cereals-desktop/internal/relations"
	"github.com/cereals-af/cereals-desktop/internal/resource"
	"github.com/cereals-af/cereals-desktop/internal/resourceform"
	"github.com/cereals-af/cereals-desktop/internal/schema"
	"github.com/cereals-af/cereals-desktop/internal/session"
	"github.com/cereals-af/cereals-desktop/internal/storage"
	"github.com/cereals-af/cereals-desktop/internal/workload"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

type nativePicker struct{ window application.Window }

func (p *nativePicker) PickKubeconfig() (string, error) {
	return application.Get().Dialog.OpenFile().AttachToWindow(p.window).SetTitle("导入 kubeconfig").ShowHiddenFiles(true).PromptForSingleSelection()
}

func (p *nativePicker) SaveFile(name string) (string, error) {
	return application.Get().Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{Title: "保存到本地", Filename: name, CanCreateDirectories: true, Window: p.window}).PromptForSingleSelection()
}

func (p *nativePicker) PickUpload() (string, error) {
	return application.Get().Dialog.OpenFile().AttachToWindow(p.window).SetTitle("选择要上传的文件").PromptForSingleSelection()
}

func main() {
	// The replacement helper must run before config, cluster or session startup.
	updater.HandleHelperMode()
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal("无法定位应用配置目录")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appConfigDir, err := localstore.ConfigDirectory(configDir, os.Getenv("CEREALS_CONFIG_DIR"), os.Getenv("CREALS_CONFIG_DIR"))
	if err != nil {
		log.Fatal(err)
	}
	store := localstore.NewSources(filepath.Join(appConfigDir, "sources.json"))
	manager := cluster.NewManager(ctx, kube.Factory{}, store, localstore.NewConnectionPreferences(filepath.Join(appConfigDir, "connection.json")))
	manifests := manifest.NewService(manager)
	podSessions := session.NewService(ctx, manager)
	picker := &nativePicker{}
	fileTransfers := filetransfer.NewService(ctx, podSessions, manager, picker)
	watches := observe.NewService(ctx, manager)
	var shell atomic.Pointer[desktop.Shell]
	app := application.New(application.Options{
		Name: "cereals", Description: "本地 Kubernetes 运维工作台",
		Icon:       desktop.AppIcon,
		OnShutdown: cancel,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: desktop.InstanceID(appConfigDir),
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if current := shell.Load(); current != nil {
					current.Show()
				}
			},
		},
		Services: []application.Service{
			application.NewService(desktop.NewWorkspaceService(manager, picker)),
			application.NewService(resource.NewService(manager)),
			application.NewService(overview.NewService(manager)),
			application.NewService(metrics.NewService(manager)),
			application.NewService(resourceform.NewService(manager, manifests)),
			application.NewService(manifests),
			application.NewService(node.NewService(ctx, manager, manifests)),
			application.NewService(storage.NewService(manager, manifests)),
			application.NewService(customresource.NewService(manager, manifests)),
			application.NewService(workload.NewService(manager, manifests)),
			application.NewService(network.NewService(manager, manifests)),
			application.NewService(configuration.NewService(manager, manifests)),
			application.NewService(watches),
			application.NewService(schema.NewService(manager)),
			application.NewService(relations.NewService(manager)),
			application.NewService(podSessions),
			application.NewService(fileTransfers),
			application.NewService(desktop.NewSessionFilesService(podSessions, picker)),
		},
		Assets:  application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:     application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
	})
	provider, err := appupdate.NewProvider()
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: buildinfo.Version,
		Providers:      []updater.Provider{provider},
		Window:         &updater.BuiltinWindow{Options: updater.WindowOptions{Title: "cereals · 软件更新"}},
	}); err != nil {
		log.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		log.Fatal("无法定位当前应用程序: ", err)
	}
	updatesEnabled := appupdate.Enabled && appupdate.SupportsInstallation(runtime.GOOS, executable)
	updates := appupdate.NewService(ctx, app.Updater, updatesEnabled, func(err error) {
		app.Logger.Warn("自动检查更新失败", "error", err)
	})
	app.RegisterService(application.NewService(updates))
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "cereals · Kubernetes 工作台", Width: 1440, Height: 940, MinWidth: 1000, MinHeight: 700,
		BackgroundColour: application.NewRGB(244, 247, 247), URL: "/",
	})
	// Set once before Run starts dispatching bindings; dialogs stay with their owner.
	picker.window = window
	preferences := localstore.NewTrayPreferences(filepath.Join(appConfigDir, "tray.json"), runtime.GOOS != "linux")
	shell.Store(desktop.NewShell(app, window, manager, preferences, updates.CheckForUpdates, func(hidden bool) {
		podSessions.SetBackground(hidden)
		fileTransfers.SetBackground(hidden)
		watches.SetBackground(hidden)
	}))
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
