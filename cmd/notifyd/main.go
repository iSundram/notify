package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/iSundram/notify/internal/config"
	"github.com/iSundram/notify/internal/model"
	"github.com/iSundram/notify/internal/server"
	"github.com/iSundram/notify/internal/store"
)

func main() {
	configPath := flag.String("config", "", "path to config file (YAML)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Ensure database directory exists.
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	// Ensure socket directory exists.
	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0755); err != nil {
		log.Fatalf("create socket dir: %v", err)
	}

	// Ensure cache directory exists.
	if cfg.CacheFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.CacheFile), 0755); err != nil {
			log.Fatalf("create cache dir: %v", err)
		}
	}

	// Open store.
	st, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Update unread cache file on startup.
	updateCacheFile(st, cfg.CacheFile)

	// Start Unix socket server.
	sockSrv, err := server.NewSocketServer(
		&cacheUpdatingStore{Store: st, cacheFile: cfg.CacheFile},
		cfg.SocketPath,
	)
	if err != nil {
		log.Fatalf("start socket server: %v", err)
	}
	go func() {
		if err := sockSrv.Serve(); err != nil {
			log.Printf("socket server stopped: %v", err)
		}
	}()
	log.Printf("socket server listening on %s", cfg.SocketPath)

	// Start HTTP server.
	httpSrv := server.NewHTTPServer(&cacheUpdatingStore{Store: st, cacheFile: cfg.CacheFile})
	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpSrv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	sockSrv.Close()
	log.Println("stopped")
}

// cacheUpdatingStore wraps a Store and updates the unread count cache file
// after every mutation.
type cacheUpdatingStore struct {
	store.Store
	cacheFile string
}

func (c *cacheUpdatingStore) Create(n *model.Notification) (string, error) {
	id, err := c.Store.Create(n)
	if err == nil {
		updateCacheFile(c.Store, c.cacheFile)
	}
	return id, err
}

func (c *cacheUpdatingStore) MarkRead(id, readBy string) error {
	err := c.Store.MarkRead(id, readBy)
	if err == nil {
		updateCacheFile(c.Store, c.cacheFile)
	}
	return err
}

func (c *cacheUpdatingStore) MarkUnread(id string) error {
	err := c.Store.MarkUnread(id)
	if err == nil {
		updateCacheFile(c.Store, c.cacheFile)
	}
	return err
}

func (c *cacheUpdatingStore) MarkAllRead(readBy string) error {
	err := c.Store.MarkAllRead(readBy)
	if err == nil {
		updateCacheFile(c.Store, c.cacheFile)
	}
	return err
}

func (c *cacheUpdatingStore) Delete(id string) error {
	err := c.Store.Delete(id)
	if err == nil {
		updateCacheFile(c.Store, c.cacheFile)
	}
	return err
}

func updateCacheFile(st store.Store, path string) {
	if path == "" {
		return
	}
	count, err := st.Count("unread")
	if err != nil {
		log.Printf("WARN update cache: %v", err)
		return
	}
	data := []byte(strconv.Itoa(count))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("WARN write cache tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("WARN rename cache: %v", err)
	}
}
