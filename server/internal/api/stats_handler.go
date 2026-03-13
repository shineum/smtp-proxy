package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sungwon/smtp-proxy/server/internal/auth"
	"github.com/sungwon/smtp-proxy/server/internal/storage"
)

// parseDateRange extracts from/to query parameters as pgtype.Timestamptz.
// Defaults to last 30 days if not provided.
func parseDateRange(r *http.Request) (pgtype.Timestamptz, pgtype.Timestamptz) {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	return pgtype.Timestamptz{Time: from, Valid: true},
		pgtype.Timestamptz{Time: to, Valid: true}
}

// isSystemAdmin returns true if the authenticated user belongs to a system group.
func isSystemAdmin(r *http.Request) bool {
	return auth.GroupTypeFromContext(r.Context()) == "system"
}

// filterGroupIDs extracts optional group_id query parameters for admin filtering.
// Accepts a comma-separated list of integer IDs: ?group_id=1,2,3
// Returns the parsed IDs and true if at least one valid ID is present.
func filterGroupIDs(r *http.Request) ([]int32, bool) {
	v := r.URL.Query().Get("group_id")
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ",")
	var ids []int32
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, err := strconv.ParseInt(p, 10, 32); err == nil {
			ids = append(ids, int32(n))
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

// dashboardResponse contains aggregated stats for the dashboard.
type dashboardResponse struct {
	TotalMessages int32            `json:"total_messages"`
	StatusCounts  map[string]int32 `json:"status_counts"`
	SuccessRate   float64          `json:"success_rate"`
	From          string           `json:"from"`
	To            string           `json:"to"`
	GroupID       string           `json:"group_id"`
	GroupName     string           `json:"group_name"`
}

// DashboardHandler handles GET /api/v1/stats/dashboard.
// Returns aggregated delivery counts and success rate.
// System admins see all groups by default; optionally filter by group_id.
func DashboardHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromTS, toTS := parseDateRange(r)

		var rows []storage.CountDeliveryLogsByGroupDateRangeRow
		var err error
		respGroupID := ""
		respGroupName := ""

		sysAdmin := isSystemAdmin(r)
		filterIDs, hasFilter := filterGroupIDs(r)

		if sysAdmin && !hasFilter {
			// System admin with no filter: query across all groups.
			rows, err = queries.CountAllDeliveryLogsByDateRange(r.Context(), storage.DateRangeParams{
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
			respGroupName = "All Groups"
		} else if sysAdmin && hasFilter && len(filterIDs) == 1 {
			// System admin with a single group filter.
			rows, err = queries.CountDeliveryLogsByGroupDateRange(r.Context(), storage.CountDeliveryLogsByGroupDateRangeParams{
				GroupID:     pgtype.Int4{Int32: filterIDs[0], Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
			respGroupID = fmt.Sprintf("%d", filterIDs[0])
		} else if sysAdmin && hasFilter {
			// System admin with multiple group filter.
			rows, err = queries.CountDeliveryLogsByGroupIDs(r.Context(), storage.MultiGroupDateRangeParams{
				GroupIDs:    filterIDs,
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else {
			// Non-admin: own group only.
			rows, err = queries.CountDeliveryLogsByGroupDateRange(r.Context(), storage.CountDeliveryLogsByGroupDateRangeParams{
				GroupID:     pgtype.Int4{Int32: groupID, Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
			respGroupID = fmt.Sprintf("%d", groupID)
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		statusCounts := make(map[string]int32)
		var total int32
		for _, row := range rows {
			statusCounts[row.Status] = row.Count
			total += row.Count
		}

		var successRate float64
		if total > 0 {
			delivered := statusCounts["delivered"]
			successRate = float64(delivered) / float64(total) * 100
		}

		respondJSON(w, http.StatusOK, dashboardResponse{
			TotalMessages: total,
			StatusCounts:  statusCounts,
			SuccessRate:   successRate,
			From:          fromTS.Time.Format(time.RFC3339),
			To:            toTS.Time.Format(time.RFC3339),
			GroupID:       respGroupID,
			GroupName:     respGroupName,
		})
	}
}

// timeSeriesPoint represents a single data point in a time series.
type timeSeriesPoint struct {
	Day    string `json:"day"`
	Status string `json:"status"`
	Count  int32  `json:"count"`
}

// TimeSeriesHandler handles GET /api/v1/stats/timeseries.
// Returns daily delivery counts by status.
// System admins see all groups by default; optionally filter by group_id.
func TimeSeriesHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromTS, toTS := parseDateRange(r)

		var rows []storage.DailyDeliveryCountsByGroupRow
		var err error

		sysAdmin := isSystemAdmin(r)
		filterIDs, hasFilter := filterGroupIDs(r)

		if sysAdmin && !hasFilter {
			rows, err = queries.DailyDeliveryCountsAll(r.Context(), storage.DateRangeParams{
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter && len(filterIDs) == 1 {
			rows, err = queries.DailyDeliveryCountsByGroup(r.Context(), storage.DailyDeliveryCountsByGroupParams{
				GroupID:     pgtype.Int4{Int32: filterIDs[0], Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter {
			rows, err = queries.DailyDeliveryCountsByGroupIDs(r.Context(), storage.MultiGroupDateRangeParams{
				GroupIDs:    filterIDs,
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else {
			rows, err = queries.DailyDeliveryCountsByGroup(r.Context(), storage.DailyDeliveryCountsByGroupParams{
				GroupID:     pgtype.Int4{Int32: groupID, Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		points := make([]timeSeriesPoint, len(rows))
		for i, row := range rows {
			day := ""
			if row.Day.Valid {
				day = row.Day.Time.Format("2006-01-02")
			}
			points[i] = timeSeriesPoint{
				Day:    day,
				Status: row.Status,
				Count:  row.Count,
			}
		}

		respondJSON(w, http.StatusOK, points)
	}
}

// usageByUserRow represents per-user delivery counts.
type usageByUserRow struct {
	UserID string `json:"user_id"`
	Status string `json:"status"`
	Count  int32  `json:"count"`
}

// UsageByUserHandler handles GET /api/v1/stats/by-user.
// Returns per-user delivery counts.
// System admins see all groups by default; optionally filter by group_id.
func UsageByUserHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromTS, toTS := parseDateRange(r)

		var rows []storage.DeliveryCountsByGroupAndUserRow
		var err error

		sysAdmin := isSystemAdmin(r)
		filterIDs, hasFilter := filterGroupIDs(r)

		if sysAdmin && !hasFilter {
			rows, err = queries.DeliveryCountsByUserAll(r.Context(), storage.DateRangeParams{
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter && len(filterIDs) == 1 {
			rows, err = queries.DeliveryCountsByGroupAndUser(r.Context(), storage.DeliveryCountsByGroupAndUserParams{
				GroupID:     pgtype.Int4{Int32: filterIDs[0], Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter {
			rows, err = queries.DeliveryCountsByUserAndGroupIDs(r.Context(), storage.MultiGroupDateRangeParams{
				GroupIDs:    filterIDs,
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else {
			rows, err = queries.DeliveryCountsByGroupAndUser(r.Context(), storage.DeliveryCountsByGroupAndUserParams{
				GroupID:     pgtype.Int4{Int32: groupID, Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		result := make([]usageByUserRow, len(rows))
		for i, row := range rows {
			uid := ""
			if row.UserID.Valid {
				uid = fmt.Sprintf("%d", row.UserID.Int32)
			}
			result[i] = usageByUserRow{
				UserID: uid,
				Status: row.Status,
				Count:  row.Count,
			}
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// usageByProviderRow represents per-provider delivery counts.
type usageByProviderRow struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Count    int32  `json:"count"`
}

// UsageByProviderHandler handles GET /api/v1/stats/by-provider.
// Returns per-provider delivery counts.
// System admins see all groups by default; optionally filter by group_id.
func UsageByProviderHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		fromTS, toTS := parseDateRange(r)

		var rows []storage.DeliveryCountsByGroupAndProviderRow
		var err error

		sysAdmin := isSystemAdmin(r)
		filterIDs, hasFilter := filterGroupIDs(r)

		if sysAdmin && !hasFilter {
			rows, err = queries.DeliveryCountsByProviderAll(r.Context(), storage.DateRangeParams{
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter && len(filterIDs) == 1 {
			rows, err = queries.DeliveryCountsByGroupAndProvider(r.Context(), storage.DeliveryCountsByGroupAndProviderParams{
				GroupID:     pgtype.Int4{Int32: filterIDs[0], Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else if sysAdmin && hasFilter {
			rows, err = queries.DeliveryCountsByProviderAndGroupIDs(r.Context(), storage.MultiGroupDateRangeParams{
				GroupIDs:    filterIDs,
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		} else {
			rows, err = queries.DeliveryCountsByGroupAndProvider(r.Context(), storage.DeliveryCountsByGroupAndProviderParams{
				GroupID:     pgtype.Int4{Int32: groupID, Valid: true},
				CreatedAt:   fromTS,
				CreatedAt_2: toTS,
			})
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		result := make([]usageByProviderRow, len(rows))
		for i, row := range rows {
			provider := ""
			if row.Provider.Valid {
				provider = row.Provider.String
			}
			result[i] = usageByProviderRow{
				Provider: provider,
				Status:   row.Status,
				Count:    row.Count,
			}
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// usageByGroupRow represents per-group delivery counts.
type usageByGroupRow struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Status    string `json:"status"`
	Count     int32  `json:"count"`
}

// UsageByGroupHandler handles GET /api/v1/stats/by-group.
// Returns per-group delivery counts. Only available for system admins.
func UsageByGroupHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := auth.GroupIDFromContext(r.Context())
		if groupID == 0 {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if !isSystemAdmin(r) {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}

		fromTS, toTS := parseDateRange(r)

		rows, err := queries.DeliveryCountsByGroupAll(r.Context(), storage.DateRangeParams{
			CreatedAt:   fromTS,
			CreatedAt_2: toTS,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		result := make([]usageByGroupRow, len(rows))
		for i, row := range rows {
			result[i] = usageByGroupRow{
				GroupID:   fmt.Sprintf("%d", row.GroupID),
				GroupName: row.GroupName,
				Status:    row.Status,
				Count:     row.Count,
			}
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// ProviderHealthHandler handles GET /api/v1/providers/{id}/health.
// Returns a basic health status for a provider.
func ProviderHealthHandler(queries storage.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		n, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid provider ID format")
			return
		}
		id := int32(n)

		provider, err := queries.GetProviderByID(r.Context(), id)
		if err != nil {
			respondError(w, http.StatusNotFound, "provider not found")
			return
		}

		// Check recent delivery logs for this provider
		now := time.Now().UTC()
		fromTS := pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true}
		toTS := pgtype.Timestamptz{Time: now, Valid: true}

		groupID := auth.GroupIDFromContext(r.Context())
		rows, err := queries.DeliveryCountsByGroupAndProvider(r.Context(), storage.DeliveryCountsByGroupAndProviderParams{
			GroupID:     pgtype.Int4{Int32: groupID, Valid: true},
			CreatedAt:   fromTS,
			CreatedAt_2: toTS,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		var sent, failed int32
		providerName := string(provider.ProviderType)
		for _, row := range rows {
			if row.Provider.Valid && row.Provider.String == providerName {
				switch row.Status {
				case "delivered":
					sent += row.Count
				case "failed":
					failed += row.Count
				}
			}
		}

		status := "healthy"
		if !provider.Enabled {
			status = "disabled"
		} else if failed > 0 && sent == 0 {
			status = "unhealthy"
		} else if failed > sent {
			status = "degraded"
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"provider_id": provider.ID,
			"status":      status,
			"enabled":     provider.Enabled,
			"sent_24h":    sent,
			"failed_24h":  failed,
		})
	}
}
