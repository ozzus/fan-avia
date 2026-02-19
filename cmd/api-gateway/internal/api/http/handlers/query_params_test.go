package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestParsePositiveIntQuery(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		key           string
		wantValue     string
		wantPresent   bool
		wantErrFilled bool
	}{
		{
			name:          "missing key",
			rawURL:        "/v1/matches/upcoming-with-airfare?limit=12",
			key:           "club_id",
			wantValue:     "",
			wantPresent:   false,
			wantErrFilled: false,
		},
		{
			name:          "empty value",
			rawURL:        "/v1/matches/upcoming-with-airfare?club_id=",
			key:           "club_id",
			wantValue:     "",
			wantPresent:   true,
			wantErrFilled: true,
		},
		{
			name:          "invalid value",
			rawURL:        "/v1/matches/upcoming-with-airfare?club_id=abc",
			key:           "club_id",
			wantValue:     "",
			wantPresent:   true,
			wantErrFilled: true,
		},
		{
			name:          "valid positive value",
			rawURL:        "/v1/matches/upcoming-with-airfare?club_id=001",
			key:           "club_id",
			wantValue:     "1",
			wantPresent:   true,
			wantErrFilled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.rawURL, nil)

			gotValue, gotPresent, gotErr := parsePositiveIntQuery(req, tc.key)
			if gotValue != tc.wantValue {
				t.Fatalf("expected value %q, got %q", tc.wantValue, gotValue)
			}
			if gotPresent != tc.wantPresent {
				t.Fatalf("expected present=%v, got %v", tc.wantPresent, gotPresent)
			}
			if (gotErr != "") != tc.wantErrFilled {
				t.Fatalf("expected err filled=%v, got %q", tc.wantErrFilled, gotErr)
			}
		})
	}
}
