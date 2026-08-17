package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/glebarez/sqlite"
)

// cmdStatus displays the running status of the 2panel systemd service.
// Usage: 2panel status
func cmdStatus(args []string) int {
	cliBanner("服务状态")
	cliSep()

	pid := findServicePID()
	if pid == 0 {
		cliErr("2panel 服务未运行")
		return 1
	}

	printProcStatus(pid)
	printNetStatus(pid)
	printUptime(pid)
	printMemory(pid)
	printResourceLimits(pid)

	cliSep()
	cliKV("数据目录", detectDataDir())
	cliKV("安装路径", detectStatusBinPath())

	cliSep()
	fmt.Println("")
	return 0
}

// findServicePID returns the main PID of the "2panel" systemd unit, or falls
// back to scanning /proc for a running 2panel binary.
func findServicePID() int {
	if p := pidFromSystemd(); p > 0 {
		return p
	}
	return pidFromProc()
}

// pidFromSystemd asks systemctl for the main PID.
func pidFromSystemd() int {
	raw := execOutput("systemctl", "show", "-p", "MainPID", "2panel")
	if raw == "" {
		return 0
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "MainPID=") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "MainPID="))
			if pid, err := strconv.Atoi(v); err == nil && pid > 0 {
				if dirExists(fmt.Sprintf("/proc/%d", pid)) {
					return pid
				}
			}
		}
	}
	return 0
}

// pidFromProc scans /proc for a process whose executable is "2panel".
func pidFromProc() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() || !isNumericStr(e.Name()) {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue
		}
		if filepath.Base(target) == "2panel" {
			return pid
		}
	}
	return 0
}

// printProcStatus displays PID and thread count from /proc/<pid>/status.
func printProcStatus(pid int) {
	status := readProcStatus(pid)
	threads := statusField(status, "Threads")

	cliSection("进程信息")
	cliKV("进程 PID", strconv.Itoa(pid))
	cliKV("线程数量", threads)
}

// printNetStatus resolves the primary listening port of the given process.
func printNetStatus(pid int) {
	port := findListenPort(pid)
	cliSection("网络")
	if port != "" {
		cliKV("监听端口", port)
	} else {
		cliKV("监听端口", "未找到")
	}
}

// findListenPort determines the listening port belonging to pid by
// cross-referencing socket inodes in /proc/<pid>/fd with /proc/<pid>/net/tcp.
// /proc/<pid>/net/tcp alone shows ALL system sockets, not just the process's.
func findListenPort(pid int) string {
	// Step 1: collect socket inodes owned by this process.
	processInodes := collectSocketInodes(pid)
	if len(processInodes) == 0 {
		return ""
	}

	// Step 2: parse /proc/<pid>/net/tcp and tcp6, match by inode.
	for _, proto := range []string{"tcp", "tcp6"} {
		if port := findPortInNetTCP(pid, proto, processInodes); port != "" {
			return port
		}
	}
	return ""
}

// collectSocketInodes reads /proc/<pid>/fd/* symlinks and returns the set of
// inode numbers for socket-type file descriptors (e.g. "socket:[12345]").
func collectSocketInodes(pid int) map[int64]bool {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	inodes := make(map[int64]bool)
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, e.Name()))
		if err != nil {
			continue
		}
		// Socket links look like "socket:[12345678]"
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
			num := strings.TrimPrefix(target, "socket:[")
			num = strings.TrimSuffix(num, "]")
			if v, err := strconv.ParseInt(num, 10, 64); err == nil {
				inodes[v] = true
			}
		}
	}
	return inodes
}

// findPortInNetTCP parses /proc/<pid>/net/<proto> and returns the local port
// of the first LISTEN entry whose inode belongs to processInodes.
//
// /proc/net/tcp format (hex):
//
//	  sl  local_address rem_address   st tx_queue rx_queue ...  inode
//	  0: 0100007F:1F90 00000000:0000 0A ... 12345678
//
// Field index 9 = inode (0-based from the split).
func findPortInNetTCP(pid int, proto string, processInodes map[int64]bool) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/net/%s", pid, proto))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := 1; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 10 {
			continue
		}
		// State "0A" = TCP_LISTEN.
		if fields[3] != "0A" {
			continue
		}
		inode, err := strconv.ParseInt(fields[9], 10, 64)
		if err != nil {
			continue
		}
		if !processInodes[inode] {
			continue
		}
		// local_address is field[1], format "HEXIP:HEXPORT".
		parts := strings.Split(fields[1], ":")
		if len(parts) < 2 {
			continue
		}
		port64, err := strconv.ParseInt(parts[1], 16, 64)
		if err != nil || port64 <= 0 {
			continue
		}
		return strconv.FormatInt(port64, 10)
	}
	return ""
}

// printUptime shows how long the process has been running.
func printUptime(pid int) {
	sec := processUptimeSec(pid)
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60

	cliSection("运行时间")
	if sec > 0 {
		cliKV("已运行", fmt.Sprintf("%d小时 %d分钟 %d秒", h, m, s))
	} else {
		cliKV("已运行", "未知")
	}
}

// processUptimeSec returns the elapsed seconds since the process started,
// computed from /proc/<pid>/stat (start time in clock ticks) and /proc/uptime.
func processUptimeSec(pid int) int64 {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	uptime, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	// The comm field (field 2) may contain spaces and parens, so find the
	// last ')' to skip past it, then parse the remaining space-separated fields.
	content := string(stat)
	closeIdx := strings.LastIndex(content, ")")
	if closeIdx < 0 || closeIdx+2 >= len(content) {
		return 0
	}
	rest := content[closeIdx+2:]
	fields := strings.Fields(rest)
	// Field 22 of /proc/<pid>/stat (after comm) is starttime (index 19 here
	// because we skipped pid(0), comm(1) and are counting from field 3).
	// Fields after comm: state(0) ppid(1) pgrp(2) session(3) tty(4) tpgid(5)
	//   flags(6) minflt(7) cminflt(8) majflt(9) cmajflt(10) utime(11)
	//   stime(12) cutime(13) cstime(14) priority(15) nice(16) numthreads(17)
	//   itrealvalue(18) starttime(19)
	if len(fields) < 20 {
		return 0
	}
	startTicks, _ := strconv.ParseInt(fields[19], 10, 64)
	clkTck := int64(100) // sysconf(_SC_CLK_TCK) — virtually always 100 on Linux
	startSec := startTicks / clkTck

	uptimeFields := strings.Fields(string(uptime))
	if len(uptimeFields) == 0 {
		return 0
	}
	uptimeSec, _ := strconv.ParseFloat(uptimeFields[0], 64)
	elapsed := int64(uptimeSec) - startSec
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// printMemory displays VmSize and VmRSS from /proc/<pid>/status.
func printMemory(pid int) {
	status := readProcStatus(pid)
	vmSize := statusField(status, "VmSize")
	vmRSS := statusField(status, "VmRSS")

	cliSection("内存")
	cliKV("虚拟内存", formatMem(vmSize))
	cliKV("物理内存", formatMem(vmRSS))
}

// formatMem converts "12345 kB" into "12345 kB (12.06 MB)".
func formatMem(raw string) string {
	if raw == "" {
		return "未知"
	}
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return raw
	}
	kb, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return raw
	}
	mb := kb / 1024.0
	if mb >= 1024 {
		gb := mb / 1024.0
		return fmt.Sprintf("%s (%.2f GB)", raw, gb)
	}
	return fmt.Sprintf("%s (%.2f MB)", raw, mb)
}

// printResourceLimits displays open file descriptor count and default password.
func printResourceLimits(pid int) {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	nFD := "?"
	if err == nil {
		nFD = strconv.Itoa(len(entries))
	}
	cliSection("资源")
	cliKV("打开文件", nFD)
	printDefaultPassword()
}

// printDefaultPassword reads the default password from the SQLite database
// and displays it if present. The password is cleared after first login.
func printDefaultPassword() {
	dataDir := detectDataDir()
	dbPath := filepath.Join(dataDir, "2panel.db")
	if _, err := os.Stat(dbPath); err != nil {
		return
	}

	dsn := dbPath +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return
	}
	defer db.Close()

	var pwd string
	err = db.QueryRow("SELECT value FROM settings WHERE key = ?", "DefaultPassword").Scan(&pwd)
	if err != nil || pwd == "" {
		return
	}
	cliKV("默认密码", pwd)
}

// ---------- helpers ----------

func readProcStatus(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	return string(data)
}

func statusField(content, name string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, name+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, name+":"))
		}
	}
	return ""
}

func isNumericStr(s string) bool {
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

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func detectStatusBinPath() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "未知"
}

func execOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}
