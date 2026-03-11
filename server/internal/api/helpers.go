package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// timestampToTime converts a pgtype.Timestamptz to time.Time.
// Returns zero time if the timestamp is not valid.
func timestampToTime(ts pgtype.Timestamptz) time.Time {
	if ts.Valid {
		return ts.Time
	}
	return time.Time{}
}

// parseAPIKeyExpiration converts an expiration preset string to a pgtype.Timestamptz.
// Valid values: "1d", "7d", "30d", "365d", "{N}d" for custom days, or "" for no expiration.
func parseAPIKeyExpiration(expiresIn string) (pgtype.Timestamptz, error) {
	if expiresIn == "" {
		return pgtype.Timestamptz{}, nil
	}
	expiresIn = strings.TrimSpace(expiresIn)
	if !strings.HasSuffix(expiresIn, "d") {
		return pgtype.Timestamptz{}, fmt.Errorf("invalid expiration format: must end with 'd' (e.g., 7d, 30d)")
	}
	daysStr := strings.TrimSuffix(expiresIn, "d")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		return pgtype.Timestamptz{}, fmt.Errorf("invalid expiration: days must be a positive integer")
	}
	return pgtype.Timestamptz{
		Time:  time.Now().AddDate(0, 0, days),
		Valid: true,
	}, nil
}

// decodeDomains unmarshals a JSON byte slice into a string slice.
// Returns an empty slice on failure or nil input.
func decodeDomains(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return []string{}
	}
	return domains
}
