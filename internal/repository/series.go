package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"goshort/internal/model"
)

const SeriesDays = 14

type DayPoint struct {
	Day    string `json:"day"`
	Clicks int64  `json:"clicks"`
}

type Referrer struct {
	Name    string  `json:"name"`
	Clicks  int64   `json:"clicks"`
	Percent float64 `json:"percent"`
}

type StatsRepo struct {
	db *gorm.DB
}

func NewStatsRepo(db *gorm.DB) *StatsRepo { return &StatsRepo{db: db} }

func since(days int) time.Time {
	return time.Now().UTC().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
}

type dayRow struct {
	Day    time.Time
	Clicks int64
}

// เติมวันที่ไม่มีคลิกให้เป็น 0 เอง เพราะ GROUP BY คืนเฉพาะวันที่มีแถว
// กราฟที่ข้ามวันว่างไปจะอ่านผิดความหมาย
func fill(rows []dayRow, days int) []DayPoint {
	byDay := map[string]int64{}
	for _, r := range rows {
		byDay[r.Day.UTC().Format("2006-01-02")] = r.Clicks
	}
	start := since(days)
	out := make([]DayPoint, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, DayPoint{Day: d, Clicks: byDay[d]})
	}
	return out
}

func (s *StatsRepo) DailyForLink(ctx context.Context, linkID uint, days int) ([]DayPoint, error) {
	var rows []dayRow
	err := s.db.WithContext(ctx).Model(&model.ClickEvent{}).
		Select("date_trunc('day', created_at) AS day, count(*) AS clicks").
		Where("link_id = ? AND created_at >= ?", linkID, since(days)).
		Group("day").Order("day").Scan(&rows).Error
	return fill(rows, days), err
}

func (s *StatsRepo) DailyForAll(ctx context.Context, days int) ([]DayPoint, error) {
	var rows []dayRow
	err := s.db.WithContext(ctx).Model(&model.ClickEvent{}).
		Select("date_trunc('day', created_at) AS day, count(*) AS clicks").
		Where("created_at >= ?", since(days)).
		Group("day").Order("day").Scan(&rows).Error
	return fill(rows, days), err
}

func (s *StatsRepo) DailyForEachLink(ctx context.Context, days int) (map[uint][]int64, error) {
	type row struct {
		LinkID uint
		Day    time.Time
		Clicks int64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&model.ClickEvent{}).
		Select("link_id, date_trunc('day', created_at) AS day, count(*) AS clicks").
		Where("created_at >= ?", since(days)).
		Group("link_id, day").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	perLink := map[uint][]dayRow{}
	for _, r := range rows {
		perLink[r.LinkID] = append(perLink[r.LinkID], dayRow{Day: r.Day, Clicks: r.Clicks})
	}

	out := map[uint][]int64{}
	for id, rs := range perLink {
		points := fill(rs, days)
		series := make([]int64, len(points))
		for i, p := range points {
			series[i] = p.Clicks
		}
		out[id] = series
	}
	return out, nil
}

func (s *StatsRepo) Referrers(ctx context.Context, linkID *uint, limit int) ([]Referrer, error) {
	type row struct {
		Referrer string
		Clicks   int64
	}
	var rows []row
	q := s.db.WithContext(ctx).Model(&model.ClickEvent{}).
		Select("referrer, count(*) AS clicks").
		Group("referrer").Order("clicks desc").Limit(limit)
	if linkID != nil {
		q = q.Where("link_id = ?", *linkID)
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	var total int64
	for _, r := range rows {
		total += r.Clicks
	}

	out := make([]Referrer, 0, len(rows))
	for _, r := range rows {
		name := r.Referrer
		if name == "" {
			name = "(direct)"
		}
		pct := 0.0
		if total > 0 {
			pct = float64(r.Clicks) / float64(total) * 100
		}
		out = append(out, Referrer{Name: name, Clicks: r.Clicks, Percent: pct})
	}
	return out, nil
}

func (s *StatsRepo) UniqueVisitors(ctx context.Context, linkID uint) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.ClickEvent{}).
		Where("link_id = ?", linkID).
		Distinct("ip_hash").Count(&n).Error
	return n, err
}

func (s *StatsRepo) RecentEvents(ctx context.Context, linkID uint, limit int) ([]model.ClickEvent, error) {
	var events []model.ClickEvent
	err := s.db.WithContext(ctx).
		Where("link_id = ?", linkID).
		Order("created_at desc").Limit(limit).Find(&events).Error
	return events, err
}
