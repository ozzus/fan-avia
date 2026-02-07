package premierliga

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	derr "github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/errors"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/domain/models"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/http/client"
	"github.com/ozzus/fan-avia/cmd/match-adapter/internal/infrastructures/premierliga/mappers"
)

type Source struct {
	client *client.Client
}

func NewSource(client *client.Client) *Source {
	return &Source{
		client: client,
	}
}

func (s *Source) FetchByID(ctx context.Context, id models.MatchID) (models.Match, error) {
	intID, err := strconv.ParseInt(string(id), 10, 64)
	if err != nil {
		return models.Match{}, fmt.Errorf("parse match id %q: %w", id, err)
	}

	resp, err := s.client.GetFullDataMatch(ctx, intID)
	if err != nil {
		if errors.Is(err, derr.ErrMatchNotFound) {
			return models.Match{}, derr.ErrMatchNotFound
		}
		if errors.Is(err, derr.ErrSourceUnavailable) {
			return models.Match{}, fmt.Errorf("get full data match: %w", derr.ErrSourceUnavailable)
		}
		return models.Match{}, fmt.Errorf("get full data match: %w", err)
	}

	return mappers.ToDomainMatch(resp)
}
