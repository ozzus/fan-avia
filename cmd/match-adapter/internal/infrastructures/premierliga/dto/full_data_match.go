package dto

type GetFullDataMatchRequest struct {
	ID int64 `json:"id"`
}

type GetFullDataMatchResponse struct {
	ID          int64  `json:"id"`
	Tournament  int64  `json:"tournament"`
	Stage       int64  `json:"stage"`
	Date        string `json:"date"`
	City        string `json:"city"`
	TicketsLink string `json:"ticketsLink"`
	Stadium     string `json:"stadium"`
	ClubHome    *int64 `json:"clubH"`
	ClubAway    *int64 `json:"clubA"`
}
