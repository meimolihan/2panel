package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/scheduler"
	"github.com/2panel-dev/2panel/internal/server"
	"github.com/2panel-dev/2panel/internal/service"
)

var (
	version = "v0.1.1"
	build   = "dev"
)

func main() {
	var (
		dataDir string
		port    int
		debug   bool
		showVer bool
	)
	flag.StringVar(&dataDir, "data", "", "data directory (db, logs, scripts)")
	flag.IntVar(&port, "port", 8080, "http listen port")
	flag.BoolVar(&debug, "debug", false, "enable gin debug mode")
	flag.BoolVar(&showVer, "version", false, "show version")
	flag.Parse()

	if showVer {
		fmt.Printf("2Panel %s (%s)\n", version, build)
		os.Exit(0)
	}

	if len(dataDir) == 0 {
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf("resolve executable path failed: %v", err)
		}
		dataDir = filepath.Join(filepath.Dir(exe), "data")
	}
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "log"), filepath.Join(dataDir, "task")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create data dir %s failed: %v", dir, err)
		}
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
	service.SeedScripts()

	addr := fmt.Sprintf(":%d", port)
	log.Printf("2Panel %s (%s) listening on %s, data dir: %s", version, build, addr, dataDir)
	srv := &http.Server{
		Addr:    addr,
		Handler: server.New(debug),
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
