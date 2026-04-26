package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"FaceitAnalyzer/internal/cache"
	"FaceitAnalyzer/internal/faceit"

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

	apiKey := os.Getenv("FACEIT_API_KEY")
	client := faceit.New(apiKey)

	player, err := client.GetPlayer("flansee")
	if err != nil {
		log.Fatalf("GetPlayer: %v", err)
	}
	pretty(player)

	history, err := client.GetMatchHistory(player.PlayerID, 3)
	if err != nil {
		log.Fatalf("GetMatchHistory: %v", err)
	}
	pretty(history)

	if len(history) > 0 {
		stats, err := client.GetMatchStats(history[0].MatchID)
		if err != nil {
			log.Fatalf("GetMatchStats: %v", err)
		}
		pretty(stats)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func pretty(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	log.Println(string(b))
}
