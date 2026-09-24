package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/v2raynix/v2raynix/internal/api"
	"github.com/v2raynix/v2raynix/internal/auth"
	"github.com/v2raynix/v2raynix/internal/cli"
	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/web"
)

var (
	Version   = "0.9.0-beta"
	BuildTime = "2026-09-24"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		targetDir := "/etc/v2raynix"
		if os.Geteuid() != 0 {
			if _, err := os.Stat("/etc/v2raynix"); err != nil {
				targetDir = "./data"
			}
		}
		if err := cli.RunSetup(targetDir, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	port := flag.Int("port", 2080, "Web UI listening port")
	dataDir := flag.String("data-dir", "", "Path to data directory (default: /etc/v2raynix or ./data)")
	initPassword := flag.String("init-password", "", "Set or reset the admin password")
	mockMode := flag.Bool("mock", false, "Run in simulation mode (without touching system network)")
	showVersion := flag.Bool("version", false, "Show V2Raynix version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("V2Raynix v%s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Resolve data directory
	targetDir := *dataDir
	if targetDir == "" {
		if os.Geteuid() == 0 {
			targetDir = "/etc/v2raynix"
		} else {
			targetDir = "./data"
		}
	}

	dbFile := filepath.Join(targetDir, "v2raynix.json")
	log.Printf("[V2Raynix] Initializing data store at: %s", dbFile)

	st, err := store.New(dbFile)
	if err != nil {
		log.Fatalf("[V2Raynix] Failed to open data store: %v", err)
	}

	// Initialize admin user if needed
	admin, _ := st.GetAdminUser()
	if *initPassword != "" {
		hash, err := auth.HashPassword(*initPassword)
		if err != nil {
			log.Fatalf("[V2Raynix] Failed to hash admin password: %v", err)
		}
		_ = st.SetAdminUser(&store.UserAccount{
			Username:     "admin",
			PasswordHash: hash,
		})
		log.Printf("[V2Raynix] Admin password successfully updated for user 'admin'")
		os.Exit(0)
	} else if admin == nil {
		hash, err := auth.HashPassword("admin")
		if err != nil {
			log.Fatalf("[V2Raynix] Failed to hash admin password: %v", err)
		}
		_ = st.SetAdminUser(&store.UserAccount{
			Username:     "admin",
			PasswordHash: hash,
		})
		log.Printf("[V2Raynix] Admin user initialized with username 'admin' and password 'admin'")
	}

	// Random JWT secret
	jwtSecret := make([]byte, 32)
	_, _ = rand.Read(jwtSecret)

	// Process supervisor
	settings, _ := st.GetSettings()
	safeModeSec := 120
	if settings != nil && settings.SafeModeSeconds > 0 {
		safeModeSec = settings.SafeModeSeconds
	}
	sup := core.NewSupervisor(st, safeModeSec, *mockMode, targetDir)

	// Embedded Static Assets
	staticFS, err := web.GetStaticFS()
	if err != nil {
		log.Printf("[V2Raynix] Warning: failed to load embedded static filesystem: %v", err)
	}

	// HTTP Router
	deps := &api.Dependencies{
		Store:      st,
		Supervisor: sup,
		JWTSecret:  jwtSecret,
		StaticFS:   staticFS,
	}
	router := api.NewRouter(deps)

	portExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portExplicit = true
		}
	})

	listenPort := cli.ResolveWebPort(*port, portExplicit, settings)
	addr := fmt.Sprintf("0.0.0.0:%d", listenPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Background server listener
	go func() {
		log.Printf("[V2Raynix] Web UI and API listening at http://%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[V2Raynix] Server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[V2Raynix] Shutting down daemon...")
	_ = sup.StopTunnel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	log.Println("[V2Raynix] Daemon exited cleanly.")
}
