package api

import (
	"encoding/json"
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
