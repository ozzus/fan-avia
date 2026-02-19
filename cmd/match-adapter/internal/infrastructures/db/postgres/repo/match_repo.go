package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Repository, error) {
	poolCfg, err := buildPoolConfig(dsn)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Repository{db: pool}, nil
}

func buildPoolConfig(dsn string) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgx pool config: %w", err)
	}
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	poolCfg.ConnConfig.StatementCacheCapacity = 0
	poolCfg.ConnConfig.DescriptionCacheCapacity = 0

	return poolCfg, nil
}

func (r *Repository) Close() {
	r.db.Close()
}

func (r *Repository) GetByID(ctx context.Context, id models.MatchID) (models.Match, error) {
	matchID, err := strconv.ParseInt(string(id), 10, 64)
	if err != nil {
		return models.Match{}, fmt.Errorf("parse match id %q: %w", id, err)
	}

	const query = `
		SELECT
			match_id,
			kickoff_utc,
			city,
			stadium,
			destination_iata,
			tickets_link,
			COALESCE(club_home_id, ''),
			COALESCE(club_away_id, '')
		FROM matches
		WHERE match_id = $1
	`

	var (
		storedID int64
		match    models.Match
	)

	err = r.db.QueryRow(ctx, query, matchID).Scan(
		&storedID,
		&match.KickoffUTC,
		&match.City,
		&match.Stadium,
		&match.DestinationIATA,
		&match.TicketsLink,
		&match.HomeTeam,
		&match.AwayTeam,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Match{}, derr.ErrMatchNotFound
		}
		return models.Match{}, fmt.Errorf("query match by id: %w", err)
	}

	match.ID = models.MatchID(strconv.FormatInt(storedID, 10))
	return match, nil
}

func (r *Repository) GetUpcoming(ctx context.Context, limit int, clubID string) ([]models.Match, error) {
	if limit <= 0 {
		limit = 10
	}
	clubID = strings.TrimSpace(clubID)

	const query = `
		SELECT
			match_id,
			kickoff_utc,
			city,
			stadium,
			destination_iata,
			tickets_link,
			COALESCE(club_home_id, ''),
			COALESCE(club_away_id, '')
		FROM matches
		WHERE kickoff_utc >= now()
		  AND ($2 = '' OR club_home_id = $2 OR club_away_id = $2)
		ORDER BY kickoff_utc ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit, clubID)
	if err != nil {
		return nil, fmt.Errorf("query upcoming matches: %w", err)
	}
	defer rows.Close()

	matches := make([]models.Match, 0, limit)
	for rows.Next() {
		var (
			storedID int64
			match    models.Match
		)

		if err := rows.Scan(
			&storedID,
			&match.KickoffUTC,
			&match.City,
			&match.Stadium,
			&match.DestinationIATA,
			&match.TicketsLink,
			&match.HomeTeam,
			&match.AwayTeam,
		); err != nil {
			return nil, fmt.Errorf("scan upcoming match: %w", err)
		}

		match.ID = models.MatchID(strconv.FormatInt(storedID, 10))
		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upcoming matches: %w", err)
	}

	return matches, nil
}

func (r *Repository) GetClubs(ctx context.Context) ([]models.Club, error) {
	const query = `
		SELECT
			club_id,
			name_ru,
			COALESCE(name_en, '')
		FROM club_dictionary
		ORDER BY name_ru ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query clubs: %w", err)
	}
	defer rows.Close()

	clubs := make([]models.Club, 0, 32)
	for rows.Next() {
		var club models.Club
		if err := rows.Scan(&club.ID, &club.NameRU, &club.NameEN); err != nil {
			return nil, fmt.Errorf("scan club: %w", err)
		}
		clubs = append(clubs, club)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clubs: %w", err)
	}

	return clubs, nil
}

func (r *Repository) Upsert(ctx context.Context, match models.Match) error {
	matchID, err := strconv.ParseInt(string(match.ID), 10, 64)
	if err != nil {
		return fmt.Errorf("parse match id %q: %w", match.ID, err)
	}

	const query = `
		INSERT INTO matches (
			match_id,
			kickoff_utc,
			city,
			stadium,
			tickets_link,
			destination_iata,
			club_home_id,
			club_away_id,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (match_id) DO UPDATE SET
			kickoff_utc = EXCLUDED.kickoff_utc,
			city = EXCLUDED.city,
			stadium = EXCLUDED.stadium,
			tickets_link = EXCLUDED.tickets_link,
			destination_iata = EXCLUDED.destination_iata,
			club_home_id = EXCLUDED.club_home_id,
			club_away_id = EXCLUDED.club_away_id,
			updated_at = now()
	`

	_, err = r.db.Exec(ctx, query,
		matchID,
		match.KickoffUTC,
		match.City,
		match.Stadium,
		match.TicketsLink,
		match.DestinationIATA,
		match.HomeTeam,
		match.AwayTeam,
	)
	if err != nil {
		return fmt.Errorf("upsert match: %w", err)
	}

	return nil
}

func (r *Repository) ResolveDestinationIATA(ctx context.Context, city string) (string, error) {
	const query = `
		SELECT iata
		FROM city_iata
		WHERE city = $1
	`

	var iata string
	err := r.db.QueryRow(ctx, query, city).Scan(&iata)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", derr.ErrCityIATANotFound
		}
		return "", fmt.Errorf("resolve city iata: %w", err)
	}

	return iata, nil
}
