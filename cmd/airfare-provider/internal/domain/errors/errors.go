package errors

import "errors"

var (
	ErrInvalidOrigin   = errors.New("invalid origin iata")
	ErrMatchNotFound   = errors.New("match not found")
	ErrSourceTemporary = errors.New("temporary source failure")
	ErrAirfareNotFound = errors.New("airfare not found")
)
