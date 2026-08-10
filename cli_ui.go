package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	styleBold   = console.StyleBold
	styleReset  = console.StyleReset
)

// cliPaint colorizes s when stdout is a terminal; piped / redirected output
// stays plain so logs remain greppable.
func cliPaint(s, style string) string {
	return console.Paint(s, style)
}

// cliBold wraps s in ANSI bold (combined with an existing color style via
// cliPaint). Falls back to plain text when stdout is piped.
func cliBold(s string) string {
	return console.Paint(s, styleBold)
}

// runeWidth reports the display width of a rune (CJK/Wide = 2) so ASCII boxes
// stay aligned even with Chinese text.
func runeWidth(r rune) int {
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6)) {
		return 2
	}
	return 1
}

// displayWidth returns the total display width of s.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// cliSuccessBox draws a green ASCII success banner around title, e.g.
//
//	  ┌────────────────────────────────┐
//	  │             ✔ 备份完成           │
//	  └────────────────────────────────┘
func cliSuccessBox(title string) {
	const width = 36
	fmt.Println("")
	fmt.Println(cliPaint("  ┌"+strings.Repeat("─", width)+"┐", styleGreen))
	content := "✔ " + title
	pad := width - displayWidth(content)
	if pad < 0 {
		pad = 0
	}
	left, right := pad/2, pad-pad/2
	fmt.Println(cliPaint("  │"+strings.Repeat(" ", left)+content+strings.Repeat(" ", right)+"│", styleGreen))
	fmt.Println(cliPaint("  └"+strings.Repeat("─", width)+"┘", styleGreen))
	fmt.Println("")
}

// cliConfirmPrompt renders a y/n confirmation prompt with the default choice
// bolded and colorized (green for the default key, red for the other).
func cliConfirmPrompt(msg string, defYes bool) string {
	yes, no := "y", "n"
	if defYes {
		yes = "Y"
	} else {
		no = "N"
	}
	return cliPaint(msg, styleWhite) + " " +
		cliBold(cliPaint(yes, styleGreen)) + "/" +
		cliBold(cliPaint(no, styleRed)) + cliPaint(":", styleGrey)
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

// cliSep prints the same 32-em-dash separator used by install.sh / uninstall.sh.
func cliSep() {
	fmt.Println(cliPaint(strings.Repeat("—", 32), styleCyan))
}

// cliSection prints a section heading in the install.sh style ("▶ 标题").
func cliSection(title string) {
	fmt.Printf("  %s %s\n", cliPaint("▶", stylePurple), title)
}

// cliKV prints an aligned key/value line, mirroring install.sh's %-14s layout.
func cliKV(key, value string) {
	fmt.Printf("  %-14s %s\n", cliPaint(key, styleBlue), cliPaint(value, styleWhite))
}

// cliBanner prints the shared ASCII-art banner with a per-command subtitle.
// Every CLI entry point (backup / restore / uninstall / install.sh) speaks the
// same visual language.
func cliBanner(subtitle string) {
	fmt.Println(cliPaint(` ____  ____                  _
|___ \|  _ \ __ _ _ __   ___| |
  __) | |_) / _`+"`"+` | '_ \ / _ \ |
 / __/|  __/ (_| | | | |  __/ |
|_____|_|   \__,_|_| |_|\___|_|`, stylePurple))
	fmt.Printf("%s - %s\n", cliPaint("2Panel", styleWhite), cliPaint(subtitle, styleCyan))
	fmt.Println("")
}

// cliSpinner renders an animated progress indicator on a TTY until done
// closes. Piped/redirected output stays clean.
func cliSpinner(done <-chan struct{}, msg string) {
	if !console.StdoutTTY() {
		<-done
		return
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r\x1b[K")
			return
		default:
		}
		fmt.Printf("\r  %s %s", cliPaint(frames[i%len(frames)], styleCyan), msg)
		i++
		time.Sleep(100 * time.Millisecond)
	}
}

// humanSize formats a byte count as a human-readable size.
func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// dirSize sums the total size of every regular file under path.
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
