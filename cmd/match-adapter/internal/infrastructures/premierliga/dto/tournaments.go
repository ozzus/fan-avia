package dto

import (
	"encoding/json"
	"fmt"
)

type GetTournamentsRequest struct {
	Type int64 `json:"type"`
}

type Tournament struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsArchive bool   `json:"archive"`
	DateFrom  string `json:"dateFrom"`
	DateTo    string `json:"dateTo"`
}

func UnmarshalTournaments(raw json.RawMessage) ([]Tournament, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var direct []Tournament
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}

	var envelope struct {
		Tournaments []Tournament `json:"tournaments"`
		Data        []Tournament `json:"data"`
		Result      []Tournament `json:"result"`
		Items       []Tournament `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		switch {
		case len(envelope.Tournaments) > 0:
			return envelope.Tournaments, nil
		case len(envelope.Data) > 0:
			return envelope.Data, nil
		case len(envelope.Result) > 0:
			return envelope.Result, nil
		case len(envelope.Items) > 0:
			return envelope.Items, nil
		}
	}

	return nil, fmt.Errorf("unsupported tournaments payload shape")
}
