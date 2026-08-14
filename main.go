package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/scheduler"
	"github.com/2panel-dev/2panel/internal/server"
	"github.com/2panel-dev/2panel/internal/service"
	"github.com/2panel-dev/2panel/internal/upgrade"
)

var (
	version   = "v1.0.0"
	build     = "dev"
	buildTime = "unknown"
)

func init() {
	flag.Usage = func() {
		fmt.Printf("%s\n", cliPaint("2Panel - 计划任务管理工具", styleCyan))
		fmt.Printf("  %s %s\n", cliPaint("用法:", styleGreen), cliPaint("2panel [选项] [backup|restore|uninstall]", styleWhite))
		fmt.Println()
		fmt.Printf("  %s\n", cliPaint("子命令:", styleYellow))
		fmt.Printf("    %-30s%s\n", cliPaint("backup [输出.zip]", styleWhite), cliPaint("备份数据目录为 zip", styleGrey))
		fmt.Printf("    %-30s%s\n", cliPaint("restore [-y] <备份.zip>", styleWhite), cliPaint("从备份 zip 还原数据目录（-y 免确认）", styleGrey))
		fmt.Printf("    %-30s%s\n", cliPaint("uninstall [-y] [--purge|--keep-data]", styleWhite), cliPaint("卸载 2Panel 并（可选）删除数据", styleGrey))
		fmt.Println()
		fmt.Printf("  %s\n", cliPaint("选项:", styleYellow))
		flag.PrintDefaults()
	}
}

func main() {
	var (
		dataDir string
		port    int
		showVer bool
	)
	flag.StringVar(&dataDir, "data", "", "数据目录（数据库、日志、脚本）")
	flag.IntVar(&port, "port", 8080, "http 监听端口")
	flag.BoolVar(&showVer, "version", false, "显示版本信息")
	flag.Parse()

	// subcommands: 2panel uninstall|backup|restore ...
	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "uninstall":
			os.Exit(cmdUninstall(args[1:]))
		case "backup":
			os.Exit(cmdBackup(args[1:], dataDir))
		case "restore":
			os.Exit(cmdRestore(args[1:], dataDir))
		}
	}

	if showVer {
		cliBanner("版本信息")
		cliKV("版本", fmt.Sprintf("%s (%s)", version, build))
		cliKV("Go 版本", runtime.Version())
		cliKV("平台", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		cliKV("编译时间", buildTime)
		os.Exit(0)
	}

	if len(dataDir) == 0 {
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("resolve executable path failed: %v", err)
		}
		dataDir = filepath.Join(filepath.Dir(exe), "data")
	}
	upgrade.SetVersion(version, build)
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "log"), filepath.Join(dataDir, "task")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Fatalf("create data dir %s failed: %v", dir, err)
		}
		// tighten permissions on pre-existing directories too
		_ = os.Chmod(dir, 0700)
	}

	if err := database.Init(filepath.Join(dataDir, "2panel.db")); err != nil {
		log.Fatalf("init database failed: %v", err)
	}
	service.InitAuth()
	scheduler.Init(dataDir)

	// restore running cron entries from db
	service.RestoreCronjobs()
	service.RestoreScriptRecords()

	// seed built-in install scripts into an empty script library
	service.SeedGroups()
	service.SeedScripts()

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("%s %s %s %s\n",
		cliPaint(time.Now().Format("2006-01-02 15:04:05"), styleGrey),
		cliPaint("2Panel", styleCyan),
		cliPaint(fmt.Sprintf("%s (%s)", version, build), styleWhite),
		cliPaint(fmt.Sprintf("listening on %s, data dir: %s", addr, dataDir), styleGrey))
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
