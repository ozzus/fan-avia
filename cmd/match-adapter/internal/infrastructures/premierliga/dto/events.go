package dto

import (
	"encoding/json"
	"fmt"
)

type GetEventsRequest struct {
	Tournament *int64 `json:"tournament,omitempty"`
	Stage      *int64 `json:"stage,omitempty"`
}

type Event struct {
	ID         int64  `json:"id"`
	Tournament int64  `json:"tournament"`
	Stage      int64  `json:"stage"`
	Date       string `json:"date"`
}

func UnmarshalEvents(raw json.RawMessage) ([]Event, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var direct []Event
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}

	var envelope struct {
		Events  []Event `json:"events"`
		Matches []Event `json:"matches"`
		Data    []Event `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		switch {
		case len(envelope.Events) > 0:
			return envelope.Events, nil
		case len(envelope.Matches) > 0:
			return envelope.Matches, nil
		case len(envelope.Data) > 0:
			return envelope.Data, nil
		}
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("decode events payload: %w", err)
	}

	for _, key := range []string{"events", "matches", "data", "items", "result"} {
		v, ok := object[key]
		if !ok {
			continue
		}
		var list []Event
		if err := json.Unmarshal(v, &list); err == nil {
			return list, nil
		}
	}

	return nil, fmt.Errorf("unsupported events payload shape")
}
