package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/server"
	"github.com/wesm/agentsview/internal/sync"
)

var version = "dev"

const (
	periodicSyncInterval = 15 * time.Minute
	watcherDebounce      = 500 * time.Millisecond
	browserPollInterval  = 100 * time.Millisecond
	browserPollAttempts  = 60
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "prune":
			runPrune(os.Args[2:])
			return
		case "serve":
			runServe(os.Args[2:])
			return
		}
	}

	runServe(os.Args[1:])
}

func runServe(args []string) {
	cfg := mustLoadConfig(args)
	database := mustOpenDB(cfg)
	defer database.Close()

	engine := sync.NewEngine(
		database, cfg.ClaudeProjectDir,
		cfg.CodexSessionsDir, "local",
	)

	runInitialSync(engine)

	stopWatcher := startFileWatcher(cfg, engine)
	defer stopWatcher()

	go startPeriodicSync(engine)

	port := server.FindAvailablePort(cfg.Port)
	if port != cfg.Port {
		fmt.Printf("Port %d in use, using %d\n", cfg.Port, port)
	}
	cfg.Port = port

	srv := server.New(cfg, database, engine)

	url := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	fmt.Printf("agentsv %s listening at %s\n", version, url)

	if !cfg.NoBrowser {
		go openBrowser(url)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func mustLoadConfig(args []string) config.Config {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	config.RegisterServeFlags(fs)
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parsing flags: %v", err)
	}

	dataDir, err := config.ResolveDataDir()
	if err != nil {
		log.Fatalf("resolving data dir: %v", err)
	}
	config.MigrateFromLegacy(dataDir)

	cfg, err := config.Load(fs)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("creating data dir: %v", err)
	}
	return cfg
}

func mustOpenDB(cfg config.Config) *db.DB {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}

	if cfg.CursorSecret != "" {
		secret, err := base64.StdEncoding.DecodeString(cfg.CursorSecret)
		if err != nil {
			log.Fatalf("invalid cursor secret: %v", err)
		}
		database.SetCursorSecret(secret)
	}

	return database
}

func runInitialSync(engine *sync.Engine) {
	fmt.Println("Running initial sync...")
	stats := engine.SyncAll(printSyncProgress)
	fmt.Printf(
		"\nSync complete: %d sessions (%d synced, %d skipped)\n",
		stats.TotalSessions, stats.Synced, stats.Skipped,
	)
}

func printSyncProgress(p sync.Progress) {
	if p.SessionsTotal > 0 {
		fmt.Printf(
			"\r  %d/%d sessions (%.0f%%) · %d messages",
			p.SessionsDone, p.SessionsTotal,
			p.Percent(), p.MessagesIndexed,
		)
	}
}

func startFileWatcher(
	cfg config.Config, engine *sync.Engine,
) func() {
	onChange := func(_ []string) {
		engine.SyncAll(nil)
	}
	watcher, err := sync.NewWatcher(watcherDebounce, onChange)
	if err != nil {
		log.Printf("warning: file watcher unavailable: %v", err)
		return func() {}
	}

	if _, err := os.Stat(cfg.ClaudeProjectDir); err == nil {
		_ = watcher.WatchRecursive(cfg.ClaudeProjectDir)
	}
	if _, err := os.Stat(cfg.CodexSessionsDir); err == nil {
		_ = watcher.WatchRecursive(cfg.CodexSessionsDir)
	}
	watcher.Start()
	return watcher.Stop
}

func startPeriodicSync(engine *sync.Engine) {
	ticker := time.NewTicker(periodicSyncInterval)
	defer ticker.Stop()
	for range ticker.C {
		log.Println("Running scheduled sync...")
		engine.SyncAll(nil)
	}
}

func openBrowser(url string) {
	for range browserPollAttempts {
		time.Sleep(browserPollInterval)
		resp, err := http.Get(url + "/api/v1/stats")
		if err == nil {
			resp.Body.Close()
			break
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32",
			"url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Run()
}
