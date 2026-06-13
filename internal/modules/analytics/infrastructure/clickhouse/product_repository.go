package clickhouse

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type UsageRepository struct {
	conn driver.Conn
}

func NewUsageRepository(conn driver.Conn) *UsageRepository {
	return &UsageRepository{conn: conn}
}

func (r *UsageRepository) GetUsageTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, interval string) (*domain.UsageTimeSeriesResult, error) {
	intervalFunc := "toStartOfDay(occurred_at)"
	switch interval {
	case "hour":
		intervalFunc = "toStartOfHour(occurred_at)"
	case "day":
		intervalFunc = "toStartOfDay(occurred_at)"
	case "week":
		intervalFunc = "toStartOfWeek(occurred_at)"
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS bucket_start,
			event_type,
			toInt64(count()) AS cnt
		FROM email_events
		WHERE workspace_id = ?
	`, intervalFunc)
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += ` GROUP BY bucket_start, event_type ORDER BY bucket_start, event_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []domain.UsageTimeSeriesBucket
	for rows.Next() {
		var b domain.UsageTimeSeriesBucket
		if err := rows.Scan(&b.BucketStart, &b.EventType, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}

	return &domain.UsageTimeSeriesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Buckets:     buckets,
	}, nil
}

func (r *UsageRepository) GetUsageFeatures(ctx context.Context, workspaceID string, from, to time.Time) (*domain.UsageFeaturesResult, error) {
	query := `
		SELECT
			toStartOfDay(occurred_at) AS bucket_start,
			multiIf(
				campaign_id != '' AND campaign_id IS NOT NULL, 'campaigns',
				'generic'
			) AS feature,
			toInt64(count(DISTINCT campaign_id)) AS active_count,
			toInt64(count()) AS event_count
		FROM email_events
		WHERE workspace_id = ?
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += ` GROUP BY bucket_start, feature ORDER BY bucket_start, feature`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.UsageFeatureRow
	for rows.Next() {
		var row domain.UsageFeatureRow
		var bucketStart time.Time
		if err := rows.Scan(&bucketStart, &row.Feature, &row.ActiveCount, &row.EventCount); err != nil {
			return nil, err
		}
		row.BucketStart = bucketStart.Format(time.RFC3339)
		out = append(out, row)
	}

	return &domain.UsageFeaturesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        out,
	}, nil
}

func (r *UsageRepository) GetRiskSignals(ctx context.Context, workspaceID string, from, to time.Time) (*domain.RiskSignalsResult, error) {
	query := `
		SELECT
			toStartOfDay(occurred_at) AS day,
			campaign_id,
			event_type,
			toInt64(count()) AS cnt
		FROM email_events
		WHERE workspace_id = ?
			AND event_type IN ('bounced', 'complained')
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += ` GROUP BY day, campaign_id, event_type ORDER BY day, campaign_id, event_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type dailyCount struct {
		day        time.Time
		campaignID string
		eventType  string
		count      int64
	}

	var daily []dailyCount
	totalByCampaign := map[string]int64{}
	campaignNames := map[string]string{}
	for rows.Next() {
		var d dailyCount
		if err := rows.Scan(&d.day, &d.campaignID, &d.eventType, &d.count); err != nil {
			return nil, err
		}
		daily = append(daily, d)
		if d.eventType == "bounced" {
			totalByCampaign[d.campaignID] += d.count
		}
		campaignNames[d.campaignID] = d.campaignID
	}

	totalQuery := `
		SELECT
			campaign_id,
			toInt64(count()) AS total
		FROM email_events
		WHERE workspace_id = ?
			AND event_type = 'delivered'
	`
	totalArgs := []any{workspaceID}
	if !from.IsZero() {
		totalQuery += " AND occurred_at >= ?"
		totalArgs = append(totalArgs, from)
	}
	if !to.IsZero() {
		totalQuery += " AND occurred_at <= ?"
		totalArgs = append(totalArgs, to)
	}
	totalQuery += ` GROUP BY campaign_id`

	totalRows, err := r.conn.Query(ctx, totalQuery, totalArgs...)
	if err != nil {
		return nil, err
	}
	defer totalRows.Close()

	deliveredByCampaign := map[string]int64{}
	for totalRows.Next() {
		var campaignID string
		var total int64
		if err := totalRows.Scan(&campaignID, &total); err != nil {
			return nil, err
		}
		deliveredByCampaign[campaignID] = total
	}

	var signals []domain.RiskSignalRow
	for campaignID, total := range totalByCampaign {
		delivered := deliveredByCampaign[campaignID]
		rate := domain.ComputeRate(total, delivered)

		signalType := "bounce_rate"
		threshold := 5.0
		if strings.Contains(campaignID, "bounced") {
			_ = signalType
		}
		if rate > threshold {
			signals = append(signals, domain.RiskSignalRow{
				SignalType: "high_bounce_rate",
				Severity:   severityLevel(rate, threshold),
				Metric:     "bounce_rate",
				Value:      rate,
				Threshold:  threshold,
				DetectedAt: time.Now().Format(time.RFC3339),
				CampaignID: campaignID,
			})
		}
	}

	for _, d := range daily {
		if d.eventType == "complained" {
			delivered := deliveredByCampaign[d.campaignID]
			complaintRate := domain.ComputeRate(d.count, delivered)
			threshold := 0.1
			if complaintRate > threshold {
				signals = append(signals, domain.RiskSignalRow{
					SignalType: "high_complaint_rate",
					Severity:   severityLevel(complaintRate, threshold),
					Metric:     "complaint_rate",
					Value:      complaintRate,
					Threshold:  threshold,
					DetectedAt: d.day.Format(time.RFC3339),
					CampaignID: d.campaignID,
				})
			}
		}
	}

	if signals == nil {
		signals = []domain.RiskSignalRow{}
	}

	return &domain.RiskSignalsResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Signals:     signals,
	}, nil
}

func severityLevel(value, threshold float64) string {
	ratio := value / threshold
	switch {
	case ratio >= 10:
		return "critical"
	case ratio >= 5:
		return "high"
	case ratio >= 2:
		return "medium"
	default:
		return "low"
	}
}

func (r *UsageRepository) GetSendVolumeForecast(ctx context.Context, workspaceID string, from, to time.Time) (*domain.SendVolumeForecastResult, error) {
	query := `
		SELECT
			toStartOfDay(occurred_at) AS day,
			toInt64(count()) AS total
		FROM email_events
		WHERE workspace_id = ?
			AND event_type = 'queued'
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += ` GROUP BY day ORDER BY day`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dailyVolumes []int64
	var days []time.Time
	for rows.Next() {
		var day time.Time
		var total int64
		if err := rows.Scan(&day, &total); err != nil {
			return nil, err
		}
		dailyVolumes = append(dailyVolumes, total)
		days = append(days, day)
	}

	if len(dailyVolumes) < 2 {
		return &domain.SendVolumeForecastResult{
			Status:      "pending",
			WorkspaceID: workspaceID,
			Rows:        []domain.SendVolumeForecastRow{},
		}, nil
	}

	var sum, sumSq float64
	for _, v := range dailyVolumes {
		fv := float64(v)
		sum += fv
		sumSq += fv * fv
	}
	n := float64(len(dailyVolumes))
	mean := sum / n
	variance := (sumSq - (sum*sum)/n) / (n - 1)
	stddev := math.Sqrt(variance)

	nextDay := days[len(days)-1].Add(24 * time.Hour)

	var forecasts []domain.SendVolumeForecastRow
	for i := 0; i < 7; i++ {
		bucketStart := nextDay.Add(time.Duration(i) * 24 * time.Hour)
		mid := int64(math.Round(mean))
		low := int64(math.Round(mean - 1.96*stddev))
		high := int64(math.Round(mean + 1.96*stddev))
		if low < 0 {
			low = 0
		}

		forecasts = append(forecasts, domain.SendVolumeForecastRow{
			BucketStart:  bucketStart.Format(time.RFC3339),
			ForecastLow:  low,
			ForecastMid:  mid,
			ForecastHigh: high,
			Confidence:   0.95,
		})
	}

	return &domain.SendVolumeForecastResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Rows:        forecasts,
	}, nil
}

func (r *UsageRepository) ListDistinctWorkspaces(ctx context.Context, since time.Time) ([]string, error) {
	query := `SELECT DISTINCT workspace_id FROM email_events WHERE occurred_at >= ?`
	rows, err := r.conn.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []string
	for rows.Next() {
		var ws string
		if err := rows.Scan(&ws); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (r *UsageRepository) GetAnomalies(ctx context.Context, workspaceID string, from, to time.Time) (*domain.AnomaliesResult, error) {
	query := `
		SELECT
			toStartOfDay(occurred_at) AS day,
			event_type,
			toInt64(count()) AS cnt
		FROM email_events
		WHERE workspace_id = ?
	`
	args := []any{workspaceID}

	if !from.IsZero() {
		query += " AND occurred_at >= ?"
		args = append(args, from)
	}
	if !to.IsZero() {
		query += " AND occurred_at <= ?"
		args = append(args, to)
	}

	query += ` GROUP BY day, event_type ORDER BY day, event_type`

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type dayEvent struct {
		day       time.Time
		eventType string
		count     int64
	}

	var daily []dayEvent
	eventAverages := map[string]struct {
		sum   float64
		count int
	}{}

	for rows.Next() {
		var d dayEvent
		if err := rows.Scan(&d.day, &d.eventType, &d.count); err != nil {
			return nil, err
		}
		daily = append(daily, d)
		ea := eventAverages[d.eventType]
		ea.sum += float64(d.count)
		ea.count++
		eventAverages[d.eventType] = ea
	}

	eventStddevs := map[string]float64{}
	for eventType, ea := range eventAverages {
		mean := ea.sum / float64(ea.count)
		var sqSum float64
		for _, d := range daily {
			if d.eventType == eventType {
				diff := float64(d.count) - mean
				sqSum += diff * diff
			}
		}
		variance := sqSum / float64(ea.count)
		eventStddevs[eventType] = math.Sqrt(variance)
	}

	var anomalies []domain.AnomalyRow
	for _, d := range daily {
		ea := eventAverages[d.eventType]
		mean := ea.sum / float64(ea.count)
		stddev := eventStddevs[d.eventType]
		if stddev == 0 {
			continue
		}
		deviation := (float64(d.count) - mean) / stddev
		if math.Abs(deviation) < 3.0 {
			continue
		}

		anomalyType := "volume_spike"
		severity := "medium"
		if math.Abs(deviation) >= 5 {
			severity = "high"
		} else if math.Abs(deviation) < 4 {
			severity = "low"
		}

		dayEnd := d.day.Add(24*time.Hour - time.Second)

		anomalies = append(anomalies, domain.AnomalyRow{
			AnomalyID:   fmt.Sprintf("anomaly_%s_%s_%s", workspaceID, d.eventType, d.day.Format("20060102")),
			AnomalyType: anomalyType,
			Severity:    severity,
			Metric:      d.eventType + "_count",
			Observed:    float64(d.count),
			Expected:    math.Round(mean),
			Deviation:   math.Round(deviation*100) / 100,
			DetectedAt:  time.Now().Format(time.RFC3339),
			WindowStart: d.day.Format(time.RFC3339),
			WindowEnd:   dayEnd.Format(time.RFC3339),
		})
	}

	if anomalies == nil {
		anomalies = []domain.AnomalyRow{}
	}

	return &domain.AnomaliesResult{
		Status:      "ready",
		WorkspaceID: workspaceID,
		Anomalies:   anomalies,
	}, nil
}
