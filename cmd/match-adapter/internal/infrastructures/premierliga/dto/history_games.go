package dto

type GetHistoryGamesRequest struct {
	ID int64 `json:"id"`
}

type GetHistoryGamesResponse struct {
	HistoryMatches HistoryMatches `json:"historyMatches"`
	LastMatches    []HistoryGame  `json:"lastMatches"`
}

type HistoryMatches struct {
	Matches int `json:"matches"`
	Win     int `json:"win"`
	Draw    int `json:"draw"`
	Loss    int `json:"loss"`
}

type HistoryGame struct {
	ID              int64   `json:"id"`
	Tournament      int64   `json:"tournament"`
	Stage           string  `json:"stage"`
	ClubHome        int64   `json:"clubH"`
	ClubAway        int64   `json:"clubA"`
	GoalHome        int     `json:"goalH"`
	GoalAway        int     `json:"goalA"`
	Date            string  `json:"date"`
	VideoReviewLink *string `json:"videoReviewLink"`
	CountVideos     int     `json:"countVideos"`
}
