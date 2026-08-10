// Package console provides ANSI terminal colors for 2Panel's command-line
// output. It is the single source of truth for the palette that install.sh /
// uninstall.sh already use (the gl_* variables), so every binary entry point
// speaks the same visual language.
//
// Colors are only emitted when stdout is attached to a real terminal; piped or
// redirected output stays plain so logs remain greppable.
package console

import (
	"os"
	"sync"
)

// ANSI 256-color palette, mirroring install.sh / uninstall.sh.
const (
	StyleGrey   = "\x1b[38;5;59m"
	StyleRed    = "\x1b[38;5;9m"
	StyleGreen  = "\x1b[38;5;10m"
	StyleYellow = "\x1b[38;5;11m"
	StyleBlue   = "\x1b[38;5;32m"
	StyleWhite  = "\x1b[38;5;15m"
	StylePurple = "\x1b[38;5;13m"
	StyleCyan   = "\x1b[38;5;14m"
	StyleReset  = "\x1b[0m"
)

var (
	once  sync.Once
	color bool
)

// ColorEnabled reports whether ANSI colors should be emitted on stdout.
func ColorEnabled() bool {
	once.Do(func() {
		color = stdoutIsTTY()
	})
	return color
}

// Paint colorizes s with style when stdout is a terminal; otherwise s is
// returned unchanged.
func Paint(s, style string) string {
	if !ColorEnabled() {
		return s
	}
	return style + s + StyleReset
}

// stdoutIsTTY reports whether stdout is attached to a real terminal.
func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}
