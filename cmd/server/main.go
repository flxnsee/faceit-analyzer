package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"FaceitAnalyzer/internal/api"
	"FaceitAnalyzer/internal/cache"
	"FaceitAnalyzer/internal/faceit"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	apiKey := os.Getenv("FACEIT_API_KEY")
	if apiKey == "" {
		log.Fatal("FACEIT_API_KEY is not set — get a key at https://developers.faceit.com")
	}

	db, err := cache.New("faceit.db")
	if err != nil {
		log.Fatalf("failed to open cache: %v", err)
	}

	fc := faceit.New(apiKey)
	h := api.New(db, fc)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(corsMiddleware)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	r.Get("/api/player/{nickname}", h.GetPlayer)
	r.Get("/api/matches/{nickname}", h.GetMatches)

	fs := http.FileServer(http.Dir("./web/static"))
	r.Handle("/*", fs)

	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		log.Println("listening on http://localhost:8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}
