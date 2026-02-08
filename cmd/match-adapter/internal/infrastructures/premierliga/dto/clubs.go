package dto

type GetClubsRequest struct {
	Tournament *int64 `json:"tournament"`
}

type Club struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	NameShort  string  `json:"nameShort"`
	Logo       string  `json:"logo"`
	Color      string  `json:"color"`
	City       string  `json:"city"`
	LogoRevert *string `json:"logoRevert"`
	Keyword    string  `json:"keyword"`
}
