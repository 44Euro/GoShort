package metrics

import (
	dto "github.com/prometheus/client_model/go"
)

type Summary struct {
	CacheHitRate float64 `json:"cache_hit_rate"`
	P99Millis    float64 `json:"p99_redirect_ms"`
	QueueDepth   int64   `json:"queue_depth"`
	QueueCap     int64   `json:"queue_capacity"`
	Dropped      int64   `json:"dropped_events"`
	Written      int64   `json:"written_events"`
	Redirects    int64   `json:"redirects"`
}

// อ่านจาก registry เดียวกับที่ /metrics เสิร์ฟ ห้ามนับซ้ำอีกทางหนึ่ง
// ไม่งั้นตัวเลขบนหน้าเว็บกับ /metrics จะไม่ตรงกันแล้วอธิบายไม่ได้ว่าอันไหนจริง
func (m *Metrics) Summary() Summary {
	families, err := m.reg.Gather()
	if err != nil {
		return Summary{}
	}

	var s Summary
	var hits, misses float64

	for _, f := range families {
		switch f.GetName() {
		case "goshort_cache_lookups_total":
			for _, mm := range f.GetMetric() {
				v := mm.GetCounter().GetValue()
				for _, l := range mm.GetLabel() {
					if l.GetName() != "result" {
						continue
					}
					if l.GetValue() == "hit" {
						hits += v
					} else {
						misses += v
					}
				}
			}
		case "goshort_click_events_dropped_total":
			s.Dropped = int64(f.GetMetric()[0].GetCounter().GetValue())
		case "goshort_click_events_written_total":
			s.Written = int64(f.GetMetric()[0].GetCounter().GetValue())
		case "goshort_click_queue_depth":
			s.QueueDepth = int64(f.GetMetric()[0].GetGauge().GetValue())
		case "goshort_click_queue_capacity":
			s.QueueCap = int64(f.GetMetric()[0].GetGauge().GetValue())
		case "goshort_redirect_duration_seconds":
			h := f.GetMetric()[0].GetHistogram()
			s.Redirects = int64(h.GetSampleCount())
			s.P99Millis = quantileFromBuckets(h, 0.99) * 1000
		}
	}

	if total := hits + misses; total > 0 {
		s.CacheHitRate = hits / total * 100
	}
	return s
}

// ค่าที่ได้เป็นการประมาณจากขอบ bucket ไม่ใช่ p99 ที่แท้จริง — ความละเอียดขึ้นกับ
// bucket ที่เลือกไว้ทั้งหมด ดู redirectBuckets
func quantileFromBuckets(h *dto.Histogram, q float64) float64 {
	total := float64(h.GetSampleCount())
	if total == 0 {
		return 0
	}
	target := q * total

	prevBound, prevCount := 0.0, 0.0
	for _, b := range h.GetBucket() {
		count := float64(b.GetCumulativeCount())
		if count >= target {
			bound := b.GetUpperBound()
			if count == prevCount {
				return bound
			}
			// interpolate เชิงเส้นใน bucket ที่ครอบ target อยู่
			return prevBound + (bound-prevBound)*(target-prevCount)/(count-prevCount)
		}
		prevBound, prevCount = b.GetUpperBound(), count
	}
	return prevBound
}
