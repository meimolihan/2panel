package main

// Sub-command "2panel uninstall": fully remove the 2Panel installation.
// Mirrors the steps of uninstall.sh so both entry points behave identically:
//   - stop & remove the systemd service (or kill background instances)
//   - remove the systemd unit file
//   - close the firewall port opened by install.sh
//   - remove the binary
//   - ask whether to delete the data directory (full uninstall)

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	uninstallBinPath     = "/usr/local/bin/2panel"
	uninstallServiceName = "2panel"
	uninstallServiceFile = "/etc/systemd/system/2panel.service"
	uninstallConfigFile  = "/etc/2panel/config"
	uninstallDefaultData = "/var/lib/2panel"
	uninstallDefaultPort = 8080
)

func cmdUninstall() int {
	if os.Geteuid() != 0 {
		fmt.Println("[错误] 请以 root 身份运行：sudo 2panel uninstall")
		return 1
	}
	printUninstallBanner()

	reader := bufio.NewReader(os.Stdin)
	if !uninstallConfirm(reader, "卸载将停止并移除 2Panel 服务与程序，是否继续？[y/N]: ", false) {
		fmt.Println("已取消卸载。")
		return 0
	}

	port, dataDir := detectUninstallConfig()

	stop2panelService()
	closeFirewallPort(port)
	remove2panelBinary()

	if uninstallConfirm(reader, fmt.Sprintf("是否删除数据目录 %s（完全卸载）？[Y/n]: ", dataDir), true) {
		if err := os.RemoveAll(dataDir); err != nil {
			fmt.Printf("[警告] 删除数据目录失败: %v\n", err)
		} else {
			fmt.Printf(">>> 已删除数据目录 %s\n", dataDir)
		}
	} else {
		fmt.Printf(">>> 已保留数据目录 %s\n", dataDir)
	}

	// remove the installation record written by install.sh
	if _, err := os.Stat(uninstallConfigFile); err == nil {
		if err := os.Remove(uninstallConfigFile); err != nil {
			fmt.Printf("[警告] 删除安装记录 %s 失败: %v\n", uninstallConfigFile, err)
		} else {
			fmt.Printf(">>> 已删除安装记录 %s\n", uninstallConfigFile)
		}
		_ = os.Remove(filepath.Dir(uninstallConfigFile))
	}

	fmt.Println("")
	fmt.Println("2Panel 已卸载完成。如需重新安装，请再次运行 install.sh 安装脚本。")
	return 0
}

func printUninstallBanner() {
	fmt.Println(` ____  ____                  _
|___ \|  _ \ __ _ _ __   ___| |
  __) | |_) / _` + "`" + ` | '_ \ / _ \ |
 / __/|  __/ (_| | | | |  __/ |
|_____|_|   \__,_|_| |_|\___|_|`)
	fmt.Println("2Panel - 卸载")
	fmt.Println("")
}

// uninstallConfirm asks a y/n question; defYes makes Enter answer yes.
func uninstallConfirm(reader *bufio.Reader, prompt string, defYes bool) bool {
	for {
		fmt.Print(prompt)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return defYes
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		switch ans {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		case "":
			return defYes
		default:
			fmt.Println("  输入无效，请输入 y 或 n。")
		}
	}
}

// detectDataDir returns the data directory of the running/installed 2panel,
// shared by uninstall/backup/restore.
func detectDataDir() string {
	_, data := detectUninstallConfig()
	return data
}

var (
	dataArgRe = regexp.MustCompile(`-data\s+(?:([^"' \t]+)|"([^"]+)"|'([^']+)')`)
	portArgRe = regexp.MustCompile(`-port\s+(\d+)`)
)

// extractDataArg pulls the value of a -data argument, tolerating quotes so a
// data directory containing spaces is parsed correctly.
func extractDataArg(s string) string {
	m := dataArgRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// readConfigFile reads the installation record written by install.sh. It is
// the authoritative source for the installed instance's port, data dir and
// binary path, so uninstall works even when no process is running and no
// systemd unit exists (the previous behavior silently left custom data dirs
// behind and a reinstall resurrected all user data).
func readConfigFile() (portS, data, bin string) {
	content, err := os.ReadFile(uninstallConfigFile)
	if err != nil {
		return "", "", ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) == 0 {
			continue
		}
		switch strings.TrimSpace(key) {
		case "PORT":
			portS = value
		case "DATA_DIR":
			data = value
		case "BIN_PATH":
			bin = value
		}
	}
	return portS, data, bin
}

// detectUninstallConfig resolves the actual port and data dir used by the
// installed 2panel. Priority: the install record written by install.sh, then
// the systemd unit file, then the cmdline of running 2panel instances, then
// defaults.
func detectUninstallConfig() (int, string) {
	port := uninstallDefaultPort
	data := uninstallDefaultData

	if portS, dir, _ := readConfigFile(); dir != "" || portS != "" {
		if portS != "" {
			if v, e := strconv.Atoi(portS); e == nil && v > 0 && v <= 65535 {
				port = v
			}
		}
		if dir != "" {
			data = dir
			return port, data
		}
	}

	if content, err := os.ReadFile(uninstallServiceFile); err == nil {
		s := string(content)
		if m := portArgRe.FindStringSubmatch(s); m != nil {
			if v, e := strconv.Atoi(m[1]); e == nil {
				port = v
			}
		}
		if d := extractDataArg(s); d != "" {
			data = d
			return port, data
		}
	}

	for _, p := range running2panelProcs() {
		if p.port != 0 {
			port = p.port
		}
		if p.data != "" {
			data = p.data
		}
		if port != uninstallDefaultPort && data != uninstallDefaultData {
			break
		}
	}
	return port, data
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func stop2panelService() {
	if _, err := os.Stat(uninstallServiceFile); err == nil {
		fmt.Println(">>> 正在停止并移除 systemd 服务 2panel ...")
		runQuiet("systemctl", "stop", uninstallServiceName)
		runQuiet("systemctl", "disable", uninstallServiceName)
		_ = os.Remove(uninstallServiceFile)
		runQuiet("systemctl", "daemon-reload")
		runQuiet("systemctl", "reset-failed")
	}
	// Always clean up background instances too, covering leftovers from a
	// stale unit file or a non-systemd install.
	stopOtherInstances()
}

// is2panelProcess reports whether the process identified by pid is an actual
// 2panel binary, checked via its resolved executable. Only the exact binary
// name matches; wrapper shells or monitoring scripts whose command line merely
// contains the path are never treated as 2panel instances.
func is2panelProcess(pid int) bool {
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return false
	}
	return filepath.Base(exe) == "2panel"
}

// stopOtherInstances terminates running 2panel processes (background mode),
// excluding the uninstall process itself.
// runningProc describes a running 2panel instance.
type runningProc struct {
	pid  int
	data string
	port int
}

// running2panelProcs lists running 2panel instances (excluding this process),
// with the port/data dir parsed from each cmdline when present.
func running2panelProcs() []runningProc {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := strconv.Itoa(os.Getpid())
	var procs []runningProc
	for _, entry := range entries {
		if !entry.IsDir() || !isNumeric(entry.Name()) || entry.Name() == self {
			continue
		}
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || !is2panelProcess(pid) {
			continue
		}
		p := runningProc{pid: pid}
		if raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline")); err == nil {
			cmd := strings.ReplaceAll(string(raw), "\x00", " ")
			if m := portArgRe.FindStringSubmatch(cmd); m != nil {
				if v, e := strconv.Atoi(m[1]); e == nil {
					p.port = v
				}
			}
			if d := extractDataArg(cmd); d != "" {
				p.data = d
			}
		}
		procs = append(procs, p)
	}
	return procs
}

// stopOtherInstances terminates every running 2panel instance.
func stopOtherInstances() {
	procs := running2panelProcs()
	if len(procs) == 0 {
		return
	}
	pids := make([]int, 0, len(procs))
	for _, p := range procs {
		pids = append(pids, p.pid)
	}
	fmt.Printf(">>> 正在停止 2panel 进程: %v ...\n", pids)
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	time.Sleep(time.Second)
	for _, pid := range pids {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func closeFirewallPort(port int) {
	portStr := strconv.Itoa(port)

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		out, _ := exec.Command("firewall-cmd", "--state").Output()
		if strings.TrimSpace(string(out)) == "running" {
			runQuiet("firewall-cmd", "--permanent", "--remove-port="+portStr+"/tcp")
			runQuiet("firewall-cmd", "--reload")
			fmt.Printf(">>> 已通过 firewalld 关闭端口 %s/tcp\n", portStr)
			return
		}
	}
	if _, err := exec.LookPath("ufw"); err == nil {
		out, _ := exec.Command("ufw", "status").Output()
		if strings.Contains(string(out), "Status: active") {
			runQuiet("ufw", "delete", "allow", portStr+"/tcp")
			fmt.Printf(">>> 已通过 ufw 关闭端口 %s/tcp\n", portStr)
			return
		}
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		if err := exec.Command("iptables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT").Run(); err == nil {
			fmt.Printf(">>> 已通过 iptables 关闭端口 %s/tcp\n", portStr)
		}
	}
}

func remove2panelBinary() {
	paths := map[string]bool{uninstallBinPath: true}
	if _, _, bin := readConfigFile(); bin != "" {
		paths[bin] = true
	}
	if exe, err := os.Executable(); err == nil {
		paths[exe] = true
	}
	for p := range paths {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			fmt.Printf("[警告] 删除二进制文件 %s 失败: %v\n", p, err)
		} else {
			fmt.Printf(">>> 已删除二进制文件 %s\n", p)
		}
	}
}

func runQuiet(name string, args ...string) {
	if _, err := exec.LookPath(name); err != nil {
		return
	}
	_ = exec.Command(name, args...).Run()
}
