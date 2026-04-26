package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"FaceitAnalyzer/internal/cache"
	"FaceitAnalyzer/internal/faceit"

	"github.com/go-chi/chi/v5"
)

const playerTTL = 300

type Handler struct {
	cache  *cache.Cache
	faceit *faceit.Client
}

func New(c *cache.Cache, f *faceit.Client) *Handler {
	return &Handler{cache: c, faceit: f}
}

type playerResponse struct {
	PlayerID   string `json:"player_id"`
	Nickname   string `json:"nickname"`
	CurrentElo int    `json:"current_elo"`
	Level      int    `json:"level"`
	Country    string `json:"country"`
	AvatarURL  string `json:"avatar_url"`
}

func (h *Handler) GetPlayer(w http.ResponseWriter, r *http.Request) {
	nickname := chi.URLParam(r, "nickname")

	cached, err := h.cache.GetPlayer(nickname)
	if err != nil {
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}
	if cached != nil && time.Now().Unix()-cached.FetchedAt < playerTTL {
		log.Printf("cache hit: player %s", nickname)
		writeJSON(w, toPlayerResponse(cached))
		return
	}

	p, err := h.faceit.GetPlayer(nickname)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	cp := &cache.Player{
		Nickname:   p.Nickname,
		PlayerID:   p.PlayerID,
		CurrentElo: p.Games.CS2.FaceitElo,
		Level:      p.Games.CS2.SkillLevel,
		Country:    p.Country,
		AvatarURL:  p.Avatar,
	}
	if err := h.cache.SavePlayer(cp); err != nil {
		log.Printf("warn: could not save player to cache: %v", err)
	}

	writeJSON(w, toPlayerResponse(cp))
}

func toPlayerResponse(p *cache.Player) playerResponse {
	return playerResponse{
		PlayerID:   p.PlayerID,
		Nickname:   p.Nickname,
		CurrentElo: p.CurrentElo,
		Level:      p.Level,
		Country:    p.Country,
		AvatarURL:  p.AvatarURL,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
