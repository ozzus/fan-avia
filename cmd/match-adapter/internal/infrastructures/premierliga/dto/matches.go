package dto

type GetMatchesRequest struct {
	Tournament int64  `json:"tournament"`
	Stage      *int64 `json:"stage,omitempty"`
}

type GetMatchesResponseItem struct {
	Stage   int64           `json:"stage"`
	Matches []MatchListItem `json:"matches"`
}

type MatchListItem struct {
	ID   int64  `json:"id"`
	Date string `json:"date"`
}
