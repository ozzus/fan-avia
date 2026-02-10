package dto

type PriceForDateItem struct {
	Price       int64  `json:"price"`
	DepartureAt string `json:"departure_at"`
	ReturnAt    string `json:"return_at"`
	DurationTo  int    `json:"duration_to"`
	Duration    int    `json:"duration"`
}

type PriceForDatesResponse struct {
	Data []PriceForDateItem `json:"data"`
}
