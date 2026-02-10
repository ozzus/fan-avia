package mappers

import (
	"sort"
	"strings"
	"time"

	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/domain/ports"
	"github.com/ozzus/fan-avia/cmd/airfare-provider/internal/infrastructures/travelpayouts/dto"
)

func ExtractPrices(data []dto.PriceForDateItem, search ports.FareSearch) []int64 {
	prices := make([]int64, 0, len(data))
	for _, item := range data {
		if item.Price <= 0 {
			continue
		}
		if !passesTimeConstraints(item, search) {
			continue
		}
		prices = append(prices, item.Price)
	}

	if len(prices) == 0 {
		return []int64{}
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	return uniqueSorted(prices)
}

func uniqueSorted(values []int64) []int64 {
	if len(values) == 0 {
		return values
	}
	result := make([]int64, 0, len(values))
	prev := values[0] - 1
	for _, v := range values {
		if v != prev {
			result = append(result, v)
			prev = v
		}
	}
	return result
}

func passesTimeConstraints(item dto.PriceForDateItem, search ports.FareSearch) bool {
	if search.ArriveNotLaterUTC == nil && search.DepartNotBeforeUTC == nil {
		return true
	}

	departure, hasDeparture := parseTime(item.DepartureAt)
	returnDate, hasReturn := parseTime(item.ReturnAt)
	arrival := time.Time{}

	if hasDeparture {
		duration := item.DurationTo
		if duration <= 0 {
			duration = item.Duration
		}
		if duration > 0 {
			arrival = departure.Add(time.Duration(duration) * time.Minute)
		}
	}

	if search.ArriveNotLaterUTC != nil {
		if arrival.IsZero() {
			return false
		}
		if arrival.After(search.ArriveNotLaterUTC.UTC()) {
			return false
		}
	}

	if search.DepartNotBeforeUTC != nil {
		base := departure
		if base.IsZero() && hasReturn {
			base = returnDate
		}
		if base.IsZero() {
			return false
		}
		if base.Before(search.DepartNotBeforeUTC.UTC()) {
			return false
		}
	}

	return true
}

func parseTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), true
		}
	}

	return time.Time{}, false
}
