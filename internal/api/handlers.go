package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"FaceitAnalyzer/internal/cache"
	"FaceitAnalyzer/internal/faceit"

	"github.com/go-chi/chi/v5"
)

const (
	playerTTL  = 300
	matchesTTL = 600
)

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
		http.Error(w, `{"error":"player not found"}`, http.StatusNotFound)
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
		log.Printf("warn: save player: %v", err)
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

type matchResponse struct {
	MatchID      string `json:"match_id"`
	Map          string `json:"map"`
	Result       string `json:"result"`
	PlayedAt     int64  `json:"played_at"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
	Headshots    int    `json:"headshots"`
	RoundsPlayed int    `json:"rounds_played"`
}

func (h *Handler) GetMatches(w http.ResponseWriter, r *http.Request) {
	nickname := chi.URLParam(r, "nickname")
	player, err := h.resolvePlayer(nickname)

	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	fresh, err := h.cache.HasFreshMatches(player.PlayerID, matchesTTL)

	if err != nil {
		log.Printf("warn: freshness check: %v", err)
	}

	if fresh {
		log.Printf("cache hit: matches %s", nickname)
		h.serveMatchesFromCache(w, player.PlayerID)
		return
	}

	history, err := h.faceit.GetMatchHistory(player.PlayerID, 100)

	if err != nil {
		http.Error(w, `{"error":"failed to fetch match history from Faceit"}`, http.StatusBadGateway)
		return
	}

	var cacheMatches []cache.Match
	var responses []matchResponse
	now := time.Now().Unix()

	for _, entry := range history {
		result := determineResult(entry, player.PlayerID)
		stats, _ := h.cache.GetMatchStats(entry.MatchID, player.PlayerID)
		mapName := ""

		if stats == nil {
			time.Sleep(50 * time.Millisecond)
			apiStats, err := h.faceit.GetMatchStats(entry.MatchID)
			if err != nil {
				log.Printf("warn: stats for %s: %v", entry.MatchID, err)
			} else {
				mapName = extractMapName(apiStats)
				ps := extractPlayerStats(apiStats, player.PlayerID)
				stats = &cache.MatchStats{
					MatchID:      entry.MatchID,
					PlayerID:     player.PlayerID,
					MapName:      mapName,
					Kills:        statInt(ps, "Kills"),
					Deaths:       statInt(ps, "Deaths"),
					Assists:      statInt(ps, "Assists"),
					Headshots:    statInt(ps, "Headshots"),
					RoundsPlayed: statInt(ps, "Rounds"),
				}
				if err := h.cache.SaveMatchStats(stats); err != nil {
					log.Printf("warn: save stats: %v", err)
				}
			}
		} else {
			mapName = stats.MapName
		}

		cacheMatches = append(cacheMatches, cache.Match{
			PlayerID:  player.PlayerID,
			MatchID:   entry.MatchID,
			GameMode:  entry.GameMode,
			MapName:   mapName,
			Result:    result,
			PlayedAt:  entry.FinishedAt,
			FetchedAt: now,
		})

		mr := matchResponse{
			MatchID:  entry.MatchID,
			Map:      mapName,
			Result:   result,
			PlayedAt: entry.FinishedAt,
		}
		if stats != nil {
			mr.Kills = stats.Kills
			mr.Deaths = stats.Deaths
			mr.Assists = stats.Assists
			mr.Headshots = stats.Headshots
			mr.RoundsPlayed = stats.RoundsPlayed
		}
		responses = append(responses, mr)
	}

	if err := h.cache.SaveMatches(cacheMatches); err != nil {
		log.Printf("warn: save matches: %v", err)
	}

	writeJSON(w, responses)
}

func (h *Handler) serveMatchesFromCache(w http.ResponseWriter, playerID string) {
	matches, err := h.cache.GetMatches(playerID)
	if err != nil {
		http.Error(w, "cache error", http.StatusInternalServerError)
		return
	}
	var responses []matchResponse
	for _, m := range matches {
		stats, _ := h.cache.GetMatchStats(m.MatchID, playerID)
		mr := matchResponse{
			MatchID:  m.MatchID,
			Map:      m.MapName,
			Result:   m.Result,
			PlayedAt: m.PlayedAt,
		}
		if stats != nil {
			mr.Map = stats.MapName
			mr.Kills = stats.Kills
			mr.Deaths = stats.Deaths
			mr.Assists = stats.Assists
			mr.Headshots = stats.Headshots
			mr.RoundsPlayed = stats.RoundsPlayed
		}
		responses = append(responses, mr)
	}
	writeJSON(w, responses)
}

func (h *Handler) resolvePlayer(nickname string) (*cache.Player, error) {
	cached, _ := h.cache.GetPlayer(nickname)
	if cached != nil && time.Now().Unix()-cached.FetchedAt < playerTTL {
		return cached, nil
	}
	p, err := h.faceit.GetPlayer(nickname)
	if err != nil {
		return nil, err
	}
	cp := &cache.Player{
		Nickname:   p.Nickname,
		PlayerID:   p.PlayerID,
		CurrentElo: p.Games.CS2.FaceitElo,
		Level:      p.Games.CS2.SkillLevel,
		Country:    p.Country,
		AvatarURL:  p.Avatar,
	}
	h.cache.SavePlayer(cp)
	return cp, nil
}

func determineResult(entry faceit.MatchEntry, playerID string) string {
	for factionName, team := range entry.Teams {
		for _, p := range team.Players {
			if p.PlayerID == playerID {
				if factionName == entry.Results.Winner {
					return "W"
				}
				return "L"
			}
		}
	}
	return "L"
}

func extractPlayerStats(resp *faceit.MatchStatsResponse, playerID string) map[string]string {
	if len(resp.Rounds) == 0 {
		return nil
	}
	for _, team := range resp.Rounds[0].Teams {
		for _, p := range team.Players {
			if p.PlayerID == playerID {
				return p.PlayerStats
			}
		}
	}
	return nil
}

func extractMapName(resp *faceit.MatchStatsResponse) string {
	if len(resp.Rounds) == 0 {
		return ""
	}
	return resp.Rounds[0].RoundStats.Map
}

func statInt(ps map[string]string, key string) int {
	if ps == nil {
		return 0
	}
	n, _ := strconv.Atoi(ps[key])
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
