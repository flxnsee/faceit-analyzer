package main

import (
	"log"
	"net/http"
	"os"

	"FaceitAnalyzer/internal/api"
	"FaceitAnalyzer/internal/cache"
	"FaceitAnalyzer/internal/faceit"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	apiKey := os.Getenv("FACEIT_API_KEY")
	if apiKey == "" {
		log.Fatal("FACEIT_API_KEY environment variable is required")
	}

	db, err := cache.New("faceit.db")
	if err != nil {
		log.Fatalf("failed to open cache: %v", err)
	}

	fc := faceit.New(apiKey)
	h := api.New(db, fc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	r.Get("/api/player/{nickname}", h.GetPlayer)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
