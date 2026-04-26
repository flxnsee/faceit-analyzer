package faceit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://open.faceit.com/data/v4"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type PlayerResponse struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
	Country  string `json:"country"`
	Avatar   string `json:"avatar"`
	Games    struct {
		CS2 struct {
			FaceitElo  int `json:"faceit_elo"`
			SkillLevel int `json:"skill_level"`
		} `json:"cs2"`
	} `json:"games"`
}

func (c *Client) GetPlayer(nickname string) (*PlayerResponse, error) {
	url := fmt.Sprintf("%s/players?nickname=%s", baseURL, nickname)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var p PlayerResponse
	return &p, json.Unmarshal(body, &p)
}

type MatchEntry struct {
	MatchID    string `json:"match_id"`
	GameMode   string `json:"game_mode"`
	FinishedAt int64  `json:"finished_at"`
	Results    struct {
		Winner string         `json:"winner"`
		Score  map[string]int `json:"score"`
	} `json:"results"`
	Teams map[string]struct {
		TeamID  string `json:"team_id"`
		Players []struct {
			PlayerID string `json:"player_id"`
		} `json:"players"`
	} `json:"teams"`
}

type historyResponse struct {
	Items []MatchEntry `json:"items"`
}

func (c *Client) GetMatchHistory(playerID string, limit int) ([]MatchEntry, error) {
	url := fmt.Sprintf("%s/players/%s/history?game=cs2&limit=%d", baseURL, playerID, limit)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var resp historyResponse
	return resp.Items, json.Unmarshal(body, &resp)
}

type MatchStatsResponse struct {
	Rounds []struct {
		RoundStats struct {
			Map    string `json:"Map"`
			Rounds string `json:"Rounds"`
			Score  string `json:"Score"`
			Winner string `json:"Winner"`
		} `json:"round_stats"`
		Teams []struct {
			TeamID  string `json:"team_id"`
			Players []struct {
				PlayerID    string            `json:"player_id"`
				PlayerStats map[string]string `json:"player_stats"`
			} `json:"players"`
		} `json:"teams"`
	} `json:"rounds"`
}

func (c *Client) GetMatchStats(matchID string) (*MatchStatsResponse, error) {
	url := fmt.Sprintf("%s/matches/%s/stats", baseURL, matchID)
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var resp MatchStatsResponse
	return &resp, json.Unmarshal(body, &resp)
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("faceit api %s: %s", resp.Status, body)
	}
	return body, nil
}
