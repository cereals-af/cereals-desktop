package desktop

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/cereals-af/cereals-desktop/internal/cluster"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type workspaceState interface{ State() cluster.State }

type nativeWindow struct{ window *application.WebviewWindow }

func (w nativeWindow) Show() {
	application.InvokeSync(func() {
		w.window.UnMinimise()
		w.window.Show()
		w.window.Focus()
	})
}

func (w nativeWindow) Hide() { application.InvokeSync(func() { w.window.Hide() }) }

// NewShell installs the native tray and window lifecycle before app.Run.
func NewShell(app *application.App, window *application.WebviewWindow, state workspaceState, preferences closePreference, checkUpdates func() error, background func(bool)) *Shell {
	keep, err := preferences.Load()
	if err != nil {
		app.Logger.Warn("无法读取托盘设置，使用默认关闭行为", "error", err)
	}
	shell := &Shell{window: nativeWindow{window}, preferences: preferences, background: background, quit: app.Quit}
	shell.closeToTray.Store(keep)
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		shell.closeRequested()
	})
	app.OnShutdown(func() { shell.stopping.Store(true) })
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(_ *application.ApplicationEvent) { shell.Show() })
	for _, event := range []events.WindowEventType{events.Common.WindowHide, events.Common.WindowShow, events.Common.WindowMinimise, events.Common.WindowUnMinimise} {
		window.OnWindowEvent(event, func(_ *application.WindowEvent) {
			application.InvokeSync(func() { background(!window.IsVisible() || window.IsMinimised()) })
		})
	}
	check := func(_ *application.Context) {
		go func() {
			if err := checkUpdates(); err != nil {
				app.Dialog.Error().SetTitle("检查更新失败").SetMessage(err.Error()).Show()
			}
		}()
	}
	installApplicationMenu(app, shell, check)

	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplate)
	} else {
		tray.SetIcon(trayIcon).SetDarkModeIcon(trayDarkIcon)
	}
	tray.SetTooltip("cereals · 本地 Kubernetes 工作台")
	menu := newTrayMenu(app, shell, check)
	menu.refresh(state.State(), window.IsVisible(), shell.closeToTray.Load())
	tray.SetMenu(menu.menu)
	openMenu := func() {
		application.InvokeSync(func() {
			if shell.stopping.Load() {
				return
			}
			snapshot := state.State()
			menu.refresh(snapshot, window.IsVisible(), shell.closeToTray.Load())
			menu.menu.Update()
			tray.SetTooltip(traySummary(snapshot))
			tray.OpenMenu()
		})
	}
	tray.OnClick(openMenu).OnRightClick(openMenu).OnDoubleClick(shell.Show)
	tray.OnMouseEnter(func() {
		if !shell.stopping.Load() {
			tray.SetTooltip(traySummary(state.State()))
		}
	})
	return shell
}

type trayMenu struct {
	menu               *application.Menu
	profiles           *application.Menu
	status, hide, keep *application.MenuItem
}

func newTrayMenu(app *application.App, shell *Shell, check func(*application.Context)) *trayMenu {
	menu := app.NewMenu()
	items := &trayMenu{menu: menu}
	menu.Add("显示主窗口").OnClick(func(_ *application.Context) { shell.Show() })
	items.hide = menu.Add("隐藏主窗口").OnClick(func(_ *application.Context) { shell.Hide() })
	menu.AddSeparator()
	items.status = menu.Add("").SetEnabled(false)
	items.profiles = menu.AddSubmenu("集群连接状态")
	menu.AddSeparator()
	items.keep = menu.AddCheckbox("关闭窗口后在托盘运行", shell.closeToTray.Load()).OnClick(func(ctx *application.Context) {
		item := ctx.ClickedMenuItem()
		if err := shell.setCloseToTray(item.Checked()); err != nil {
			item.SetChecked(shell.closeToTray.Load())
			app.Logger.Error("保存托盘设置失败", "error", err)
			app.Dialog.Error().SetTitle("无法保存托盘设置").SetMessage("设置未更改，请检查应用配置目录的写入权限。").Show()
		}
	})
	menu.Add("关于 cereals").OnClick(func(_ *application.Context) { app.Menu.ShowAbout() })
	menu.Add("检查更新…").OnClick(check)
	menu.AddSeparator()
	menu.Add("退出 cereals").OnClick(func(_ *application.Context) { shell.Quit() })
	return items
}

func (m *trayMenu) refresh(snapshot cluster.State, visible, keep bool) {
	m.hide.SetEnabled(visible)
	m.keep.SetChecked(keep)
	m.status.SetLabel(traySummary(snapshot))
	// Only read-only rows are replaced, before the native menu is displayed.
	// Destroy releases their native handles and registered menu IDs as well.
	m.profiles.Destroy()
	if len(snapshot.Profiles) == 0 {
		m.profiles.Add("尚未导入集群").SetEnabled(false)
	}
	for i, profile := range snapshot.Profiles {
		if i == 20 {
			m.profiles.Add(fmt.Sprintf("另有 %d 个，请在主窗口查看", len(snapshot.Profiles)-i)).SetEnabled(false)
			break
		}
		status := "未连接"
		if profile.Connected {
			status = "已连接"
		}
		m.profiles.Add(fmt.Sprintf("%s · %s", menuText(profile.Context), status)).SetEnabled(false)
	}
}

func installApplicationMenu(app *application.App, shell *Shell, check func(*application.Context)) {
	if runtime.GOOS != "darwin" {
		return
	}
	menu := app.NewMenu()
	main := menu.AddSubmenu("cereals")
	main.Add("关于 cereals").OnClick(func(_ *application.Context) { app.Menu.ShowAbout() })
	main.Add("检查更新…").OnClick(check)
	main.AddSeparator()
	main.AddRole(application.ServicesMenu)
	main.AddSeparator()
	main.AddRole(application.Hide)
	main.AddRole(application.HideOthers)
	main.AddRole(application.UnHide)
	main.AddSeparator()
	main.Add("退出 cereals").SetAccelerator("CmdOrCtrl+q").OnClick(func(_ *application.Context) { shell.Quit() })
	menu.AddRole(application.EditMenu)
	window := menu.AddSubmenu("窗口")
	window.Add("显示主窗口").OnClick(func(_ *application.Context) { shell.Show() })
	window.AddRole(application.Minimise)
	app.Menu.Set(menu)
}

func traySummary(state cluster.State) string {
	connected := 0
	for _, profile := range state.Profiles {
		if profile.Connected {
			connected++
		}
	}
	return fmt.Sprintf("已连接 %d / %d 个集群", connected, len(state.Profiles))
}

func menuText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, value)
}
