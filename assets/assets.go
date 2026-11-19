// Package assets carries the app's icons. scripts/make-icons.py draws them.
package assets

import _ "embed"

// TrayICO is the tray icon for Windows, which wants an .ico.
//
//go:embed icon.ico
var TrayICO []byte

// TrayPNG is the tray icon everywhere else.
//
//go:embed tray.png
var TrayPNG []byte

// TrayTemplatePNG is the macOS menu bar icon: black on transparent, which the
// menu bar recolours to suit light and dark mode.
//
//go:embed tray-template.png
var TrayTemplatePNG []byte
