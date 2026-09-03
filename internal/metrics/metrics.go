package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	reg *prometheus.Registry

	redirectDuration prometheus.Histogram
	cacheLookups     *prometheus.CounterVec
}

// bucket ปริยายของ Prometheus เริ่มที่ 5ms ซึ่งใช้กับ redirect ที่ควรจบต่ำกว่า 10ms
// ไม่ได้เลย ทุกอย่างจะกองอยู่ bucket แรกแล้ว p99 อ่านไม่ได้ความ
var redirectBuckets = []float64{
	0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1,
}

func New() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		redirectDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "goshort_redirect_duration_seconds",
			Help:    "Time spent handling GET /:code.",
			Buckets: redirectBuckets,
		}),
		cacheLookups: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "goshort_cache_lookups_total",
			Help: "Redis lookups on the redirect path, by outcome.",
		}, []string{"result"}),
	}

	m.reg.MustRegister(m.redirectDuration, m.cacheLookups)
	m.cacheLookups.WithLabelValues("hit").Add(0)
	m.cacheLookups.WithLabelValues("miss").Add(0)
	return m
}

func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// Pool เป็นสิ่งที่นับ event เองอยู่แล้ว ให้ Prometheus อ่านตอน scrape แทนที่จะ
// ให้ใครสักคนคอย set — ไม่งั้น /metrics จะรายงาน 0 จนกว่าจะมีคนเปิดหน้า dashboard
type PoolStats interface {
	Depth() int
	Capacity() int
	Dropped() int64
	Written() int64
}

func (m *Metrics) TrackPool(p PoolStats) {
	m.reg.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "goshort_click_queue_depth",
			Help: "Events currently waiting in the click buffer.",
		}, func() float64 { return float64(p.Depth()) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "goshort_click_queue_capacity",
			Help: "Capacity of the click buffer.",
		}, func() float64 { return float64(p.Capacity()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "goshort_click_events_dropped_total",
			Help: "Click events discarded because the buffer was full.",
		}, func() float64 { return float64(p.Dropped()) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "goshort_click_events_written_total",
			Help: "Click events persisted by the worker pool.",
		}, func() float64 { return float64(p.Written()) }),
	)
}

func (m *Metrics) ObserveRedirect(d time.Duration) { m.redirectDuration.Observe(d.Seconds()) }
func (m *Metrics) CacheHit()                       { m.cacheLookups.WithLabelValues("hit").Inc() }
func (m *Metrics) CacheMiss()                      { m.cacheLookups.WithLabelValues("miss").Inc() }
