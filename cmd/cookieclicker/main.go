// Command cookieclicker starts the cookie clicker game server.
// It serves the static frontend at / and handles WebSocket sessions at /ws.
//
// Required environment variable:
//
//	SESSION_SECRET  HMAC signing secret, minimum 32 bytes.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/server"
)

func main() {
	cfg := config.Default()

	ts, err := server.NewTokenService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	hub := server.NewHub(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /session", func(w http.ResponseWriter, r *http.Request) {
		server.ServeSession(ts, w, r)
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWS(hub, ts, w, r)
	})
	mux.Handle("GET /", http.FileServer(http.Dir("web/static")))

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("shutting down...")
		httpSrv.Close()
	}()

	log.Printf("cookie clicker listening on http://localhost%s", cfg.ListenAddr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
