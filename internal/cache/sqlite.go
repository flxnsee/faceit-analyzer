package cache

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Cache struct {
	db *sql.DB
}

type Player struct {
	Nickname   string
	PlayerID   string
	CurrentElo int
	Level      int
	Country    string
	AvatarURL  string
	FetchedAt  int64
}

type Match struct {
	PlayerID  string
	MatchID   string
	GameMode  string
	MapName   string
	Result    string
	EloChange int
	EloAfter  int
	PlayedAt  int64
	FetchedAt int64
}

type MatchStats struct {
	MatchID             string
	PlayerID            string
	MapName             string
	Kills               int
	Deaths              int
	Assists             int
	Headshots           int
	RoundsPlayed        int
	PistolRoundsWon     int
	PistolRoundsTotal   int
	OvertimeRoundsWon   int
	OvertimeRoundsTotal int
	First5RoundsWon     int
	First5RoundsTotal   int
	FetchedAt           int64
}

func New(path string) (*Cache, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	if err = migrate(db); err != nil {
		return nil, err
	}
	return &Cache{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			nickname    TEXT PRIMARY KEY,
			player_id   TEXT NOT NULL,
			current_elo INTEGER NOT NULL,
			level       INTEGER NOT NULL,
			country     TEXT,
			avatar_url  TEXT,
			fetched_at  INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS match_history (
			player_id  TEXT NOT NULL,
			match_id   TEXT NOT NULL,
			game_mode  TEXT,
			map_name   TEXT,
			result     TEXT,
			elo_change INTEGER,
			elo_after  INTEGER,
			played_at  INTEGER NOT NULL,
			PRIMARY KEY (player_id, match_id)
		);
		CREATE TABLE IF NOT EXISTS match_stats (
			match_id               TEXT NOT NULL,
			player_id              TEXT NOT NULL,
			kills                  INTEGER,
			deaths                 INTEGER,
			assists                INTEGER,
			headshots              INTEGER,
			rounds_played          INTEGER,
			pistol_rounds_won      INTEGER,
			pistol_rounds_total    INTEGER,
			overtime_rounds_won    INTEGER,
			overtime_rounds_total  INTEGER,
			first5_rounds_won      INTEGER,
			first5_rounds_total    INTEGER,
			fetched_at             INTEGER NOT NULL,
			PRIMARY KEY (match_id, player_id)
		);
	`)
	if err != nil {
		return err
	}

	db.Exec("ALTER TABLE match_history ADD COLUMN fetched_at INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE match_stats ADD COLUMN map_name TEXT NOT NULL DEFAULT ''")
	return nil
}

func (c *Cache) GetPlayer(nickname string) (*Player, error) {
	p := &Player{}
	err := c.db.QueryRow(`
		SELECT nickname, player_id, current_elo, level, country, avatar_url, fetched_at
		FROM players WHERE nickname = ?`, nickname,
	).Scan(&p.Nickname, &p.PlayerID, &p.CurrentElo, &p.Level, &p.Country, &p.AvatarURL, &p.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (c *Cache) SavePlayer(p *Player) error {
	p.FetchedAt = time.Now().Unix()
	_, err := c.db.Exec(`
		INSERT INTO players (nickname, player_id, current_elo, level, country, avatar_url, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(nickname) DO UPDATE SET
			player_id=excluded.player_id, current_elo=excluded.current_elo,
			level=excluded.level, country=excluded.country,
			avatar_url=excluded.avatar_url, fetched_at=excluded.fetched_at`,
		p.Nickname, p.PlayerID, p.CurrentElo, p.Level, p.Country, p.AvatarURL, p.FetchedAt,
	)
	return err
}

func (c *Cache) HasFreshMatches(playerID string, ttl int64) (bool, error) {
	var fetchedAt int64
	err := c.db.QueryRow(
		`SELECT COALESCE(MAX(fetched_at), 0) FROM match_history WHERE player_id = ?`, playerID,
	).Scan(&fetchedAt)
	if err != nil {
		return false, err
	}
	return fetchedAt > 0 && time.Now().Unix()-fetchedAt < ttl, nil
}

func (c *Cache) GetMatches(playerID string) ([]Match, error) {
	rows, err := c.db.Query(`
		SELECT player_id, match_id, game_mode, map_name, result, elo_change, elo_after, played_at, fetched_at
		FROM match_history WHERE player_id = ?
		ORDER BY played_at DESC`, playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		if err := rows.Scan(&m.PlayerID, &m.MatchID, &m.GameMode, &m.MapName,
			&m.Result, &m.EloChange, &m.EloAfter, &m.PlayedAt, &m.FetchedAt); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (c *Cache) SaveMatches(matches []Match) error {
	if len(matches) == 0 {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO match_history (player_id, match_id, game_mode, map_name, result, elo_change, elo_after, played_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(player_id, match_id) DO UPDATE SET
			map_name=excluded.map_name, result=excluded.result, fetched_at=excluded.fetched_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range matches {
		if _, err := stmt.Exec(m.PlayerID, m.MatchID, m.GameMode, m.MapName,
			m.Result, m.EloChange, m.EloAfter, m.PlayedAt, m.FetchedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Cache) GetMatchStats(matchID, playerID string) (*MatchStats, error) {
	s := &MatchStats{}
	err := c.db.QueryRow(`
		SELECT match_id, player_id, map_name, kills, deaths, assists, headshots, rounds_played,
		       pistol_rounds_won, pistol_rounds_total, overtime_rounds_won, overtime_rounds_total,
		       first5_rounds_won, first5_rounds_total, fetched_at
		FROM match_stats WHERE match_id = ? AND player_id = ?`, matchID, playerID,
	).Scan(&s.MatchID, &s.PlayerID, &s.MapName, &s.Kills, &s.Deaths, &s.Assists, &s.Headshots,
		&s.RoundsPlayed, &s.PistolRoundsWon, &s.PistolRoundsTotal,
		&s.OvertimeRoundsWon, &s.OvertimeRoundsTotal,
		&s.First5RoundsWon, &s.First5RoundsTotal, &s.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (c *Cache) SaveMatchStats(s *MatchStats) error {
	s.FetchedAt = time.Now().Unix()
	_, err := c.db.Exec(`
		INSERT INTO match_stats (
			match_id, player_id, map_name, kills, deaths, assists, headshots, rounds_played,
			pistol_rounds_won, pistol_rounds_total, overtime_rounds_won, overtime_rounds_total,
			first5_rounds_won, first5_rounds_total, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(match_id, player_id) DO NOTHING`,
		s.MatchID, s.PlayerID, s.MapName, s.Kills, s.Deaths, s.Assists, s.Headshots, s.RoundsPlayed,
		s.PistolRoundsWon, s.PistolRoundsTotal, s.OvertimeRoundsWon, s.OvertimeRoundsTotal,
		s.First5RoundsWon, s.First5RoundsTotal, s.FetchedAt,
	)
	return err
}
