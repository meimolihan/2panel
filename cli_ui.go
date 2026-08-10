package main

import (
	"fmt"

	"github.com/2panel-dev/2panel/internal/console"
)

// Style constants mirroring the install.sh / uninstall.sh palette (gl_*).
// The palette itself lives in internal/console so every entry point shares a
// single source of truth.
const (
	styleGrey   = console.StyleGrey
	styleRed    = console.StyleRed
	styleGreen  = console.StyleGreen
	styleYellow = console.StyleYellow
	styleBlue   = console.StyleBlue
	styleWhite  = console.StyleWhite
	stylePurple = console.StylePurple
	styleCyan   = console.StyleCyan
	styleReset  = console.StyleReset
)

// cliPaint colorizes s when stdout is a terminal; piped / redirected output
// stays plain so logs remain greppable.
func cliPaint(s, style string) string {
	return console.Paint(s, style)
}

func cliErr(format string, args ...interface{}) {
	fmt.Printf("  %s %s\n", cliPaint("[错误]", styleRed), fmt.Sprintf(format, args...))
}

func cliWarn(format string, args ...interface{}) {
	fmt.Printf("  %s %s\n", cliPaint("[警告]", styleYellow), fmt.Sprintf(format, args...))
}

func cliHint(format string, args ...interface{}) {
	fmt.Printf("  %s %s\n", cliPaint("[提示]", styleCyan), fmt.Sprintf(format, args...))
}

func cliOK(format string, args ...interface{}) {
	fmt.Printf("  %s %s\n", cliPaint(">>>", styleGreen), fmt.Sprintf(format, args...))
}

func cliDone(format string, args ...interface{}) {
	fmt.Printf("  %s\n", cliPaint("✔ "+fmt.Sprintf(format, args...), styleGreen))
}
