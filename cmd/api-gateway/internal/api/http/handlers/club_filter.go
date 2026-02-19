package handlers

import (
	"strings"

	matchv1 "github.com/ozzus/fan-avia/protos/gen/go/match/v1"
)

func filterMatchesByClubID(matches []*matchv1.Match, clubID string) []*matchv1.Match {
	normalizedClubID := strings.TrimSpace(clubID)
	if normalizedClubID == "" || len(matches) == 0 {
		return matches
	}

	filtered := make([]*matchv1.Match, 0, len(matches))
	for _, m := range matches {
		if m == nil {
			continue
		}

		home := strings.TrimSpace(m.GetClubHomeId())
		away := strings.TrimSpace(m.GetClubAwayId())
		if home == normalizedClubID || away == normalizedClubID {
			filtered = append(filtered, m)
		}
	}

	return filtered
}

func cutMatchesByLimit(matches []*matchv1.Match, limit int32) []*matchv1.Match {
	if limit <= 0 {
		return matches
	}
	if len(matches) <= int(limit) {
		return matches
	}
	return matches[:limit]
}
