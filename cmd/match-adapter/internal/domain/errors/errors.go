package errors

import "errors"

var (
	ErrMatchNotFound     = errors.New("match not found")
	ErrCityIATANotFound  = errors.New("city IATA not found")
	ErrSourceUnavailable = errors.New("source unavailable")
)
