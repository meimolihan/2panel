package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
)

const backupDBName = "2panel.db"

// cmdBackup packs the data dir (SQLite db + log/task) into a zip archive.
// Usage: 2panel [-data /path] backup [输出.zip]
func cmdBackup(args []string, flagData string) int {
	cliBanner("备份")

	dataDir := flagData
	if dataDir == "" {
		dataDir = detectDataDir()
	}
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		cliErr("数据目录不存在: %s", dataDir)
		return 1
	}

	dest := ""
	if len(args) > 0 {
		dest = args[0]
	}
	if dest == "" {
		dest = fmt.Sprintf("2panel-backup-%s.zip", time.Now().Format("20060102-150405"))
	}

	cliSection("备份数据目录")
	cliKV("数据目录", dataDir)
	cliKV("目标文件", dest)

	if abs, err := filepath.Abs(dest); err == nil {
		parent := filepath.Dir(abs)
		if parent == dataDir || strings.HasPrefix(parent, dataDir+string(os.PathSeparator)) {
			cliErr("备份文件不能输出到数据目录内部")
			return 1
		}
	}

	running := false
	for _, p := range running2panelProcs() {
		if p.data == dataDir {
			running = true
			break
		}
	}
	if running {
		cliHint("检测到 2panel 正在使用数据目录，将通过数据库快照保证备份一致性。")
	}

	// Snapshot the database so the archive stays consistent even while the
	// server is writing; the live -wal/-shm files are never bundled.
	dbPath := filepath.Join(dataDir, backupDBName)
	snapName := ""
	if _, err := os.Stat(dbPath); err == nil {
		cliHint("正在生成数据库快照 ...")
		snap, cErr := os.CreateTemp("", "2panel-backup-*.db")
		if cErr != nil {
			cliErr("创建临时文件失败: %v", cErr)
			return 1
		}
		snapName = snap.Name()
		snap.Close()
		defer os.Remove(snapName)
		if cErr := database.SnapshotDB(dbPath, snapName); cErr != nil {
			cliErr("生成数据库快照失败: %v", cErr)
			return 1
		}
	}

	done := make(chan struct{})
	go cliSpinner(done, "正在打包数据 ...")
	files, err := zipDir(dataDir, dest, snapName)
	close(done)
	if err != nil {
		cliErr("备份失败: %v", err)
		return 1
	}
	if abs, err := filepath.Abs(dest); err == nil {
		dest = abs
	}
	var size int64
	if st, err := os.Stat(dest); err == nil {
		size = st.Size()
	}
	cliSuccessBox("备份完成")
	cliKV("备份文件", dest)
	cliKV("文件大小", humanSize(size))
	cliKV("文件数", fmt.Sprintf("%d 个", files))
	return 0
}

// cmdRestore replaces the data dir from a backup zip produced by cmdBackup.
// Usage: 2panel [-data /path] restore [-y] <备份文件.zip>
func cmdRestore(args []string, flagData string) int {
	var backupFile string
	yes := false
	for _, a := range args {
		switch a {
		case "-y", "--yes":
			yes = true
		case "-h", "--help":
			fmt.Println(cliPaint("用法: 2panel restore [选项] <备份文件.zip>", styleGrey))
			fmt.Printf("    %-18s%s\n", cliPaint("-y, --yes", styleWhite), cliPaint("免确认，直接还原覆盖数据", styleGrey))
			fmt.Printf("    %-18s%s\n", cliPaint("-h, --help", styleWhite), cliPaint("显示帮助", styleGrey))
			return 0
		default:
			if a != "" && a[0] == '-' {
				cliErr("未知参数: %s，使用 -h 查看帮助", a)
				return 1
			}
			if backupFile != "" {
				cliErr("多余参数: %s", a)
				return 1
			}
			backupFile = a
		}
	}
	if backupFile == "" {
		fmt.Println(cliPaint("用法: 2panel restore [选项] <备份文件.zip>", styleGrey))
		return 1
	}
	if _, err := os.Stat(backupFile); err != nil {
		cliErr("备份文件不存在: %s", backupFile)
		return 1
	}

	dataDir := flagData
	if dataDir == "" {
		dataDir = detectDataDir()
	}

	if err := checkZip(backupFile); err != nil {
		cliErr("无效的备份文件: %v", err)
		return 1
	}

	cliBanner("还原")
	cliSection("还原备份")
	cliKV("备份文件", backupFile)
	cliKV("数据目录", dataDir)

	reader := bufio.NewReader(os.Stdin)
	if !yes && !uninstallConfirm(reader, fmt.Sprintf("还原将覆盖数据目录 %s 中的现有数据，是否继续？", dataDir), false) {
		fmt.Println(cliPaint("已取消还原。", styleYellow))
		return 0
	}

	cliSection("停止服务")
	// 停掉 systemd 服务防止自动重启，再兜底终止使用该数据目录的进程。
	if _, err := os.Stat(uninstallServiceFile); err == nil {
		cliOK("正在停止 2panel systemd 服务 ...")
		runQuiet("systemctl", "stop", uninstallServiceName)
	}
	stopInstanceUsingDataDir(dataDir)

	backupPath := dataDir + ".backup-" + time.Now().Format("20060102-150405")
	if isDir(dataDir) {
		if err := os.Rename(dataDir, backupPath); err != nil {
			cliWarn("备份现有数据失败: %v", err)
		} else {
			cliOK("现有数据已备份到 %s", backupPath)
		}
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		cliErr("创建数据目录失败: %v", err)
		return 1
	}

	cliSection("还原数据")
	done := make(chan struct{})
	go cliSpinner(done, "正在还原数据 ...")
	files, err := unzipDir(backupFile, dataDir)
	close(done)
	if err != nil {
		cliErr("还原失败: %v", err)
		if backupPath != dataDir && isDir(backupPath) {
			cliHint("原数据已回退到 %s，可手动恢复。", backupPath)
		}
		return 1
	}
	cliSuccessBox(fmt.Sprintf("还原完成（共 %d 个文件）", files))
	cliKV("启动命令", "systemctl start 2panel")
	return 0
}

// stopInstanceUsingDataDir terminates running 2panel instances that use the
// given data dir.
func stopInstanceUsingDataDir(dataDir string) {
	var pids []int
	for _, p := range running2panelProcs() {
		if p.data == dataDir {
			pids = append(pids, p.pid)
		}
	}
	if len(pids) == 0 {
		return
	}
	cliOK("正在停止使用数据目录 %s 的 2panel 进程: %v ...", dataDir, pids)
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

// zipDir archives src into dest, storing paths relative to src. When
// dbSnapshot is non-empty the entry for 2panel.db is served from that file
// instead of the live database. It returns the number of files archived.
func zipDir(src, dest, dbSnapshot string) (int, error) {
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()

	files := 0
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, backupDBName+"-") {
			// skip live WAL/SHM/journal companions; the snapshot is authoritative
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr := &zip.FileHeader{Name: rel + "/", Method: zip.Deflate}
			hdr.SetMode(0755)
			_, err := zw.CreateHeader(hdr)
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate
		hdr.SetMode(info.Mode())
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		file := path
		if rel == backupDBName && dbSnapshot != "" {
			file = dbSnapshot
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		_, cErr := io.Copy(w, f)
		f.Close()
		if cErr != nil {
			return cErr
		}
		files++
		return nil
	})
	return files, err
}

// checkZip validates that path is a zip archive containing a 2panel.db.
func checkZip(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("无法解压 %s（不是有效的 zip 文件？）", path)
	}
	defer r.Close()
	for _, f := range r.File {
		if filepath.Base(f.Name) == backupDBName {
			return nil
		}
	}
	return fmt.Errorf("压缩包中未找到 %s，可能不是 2Panel 备份", backupDBName)
}

// unzipDir extracts the archive into dest, guarding against zip-slip. It
// returns the number of files extracted.
func unzipDir(src, dest string) (int, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	files := 0
	for _, f := range r.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
			return 0, fmt.Errorf("压缩包中包含非法路径: %s", f.Name)
		}
		if strings.HasPrefix(name, backupDBName+"-") {
			// stale WAL/SHM/journal files would corrupt the restored database
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, dest) {
			return 0, fmt.Errorf("压缩包路径越界: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return 0, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return 0, err
		}
		rc, err := f.Open()
		if err != nil {
			return 0, err
		}
		mode := f.Mode() & 0777
		if mode == 0 {
			mode = 0600
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return 0, err
		}
		_, cErr := io.Copy(w, rc)
		rc.Close()
		w.Close()
		if cErr != nil {
			return 0, cErr
		}
		files++
	}
	return files, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
