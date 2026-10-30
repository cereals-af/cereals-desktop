package desktop

import _ "embed"

// AppIcon is the desktop artwork, also used by the native About dialog.
//
//go:embed assets/appicon.png
var AppIcon []byte

//go:embed assets/tray-template.png
var trayTemplate []byte

//go:embed assets/tray.png
var trayIcon []byte

//go:embed assets/tray-dark.png
var trayDarkIcon []byte
