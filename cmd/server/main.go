package main

import (
	"log"
	"net/http"

	"FaceitAnalyzer/internal/cache"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	db, err := cache.New("faceit.db")

	if err != nil {
		log.Fatalf("failed to open cache: %v", err)
	}

	log.Println("cache ready")
	_ = db

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
