package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type Metrics struct {
	reg *prometheus.Registry

	redirectDuration prometheus.Histogram
	cacheLookups     *prometheus.CounterVec
	dropped          prometheus.Counter
	written          prometheus.Counter
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
		dropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "goshort_click_events_dropped_total",
			Help: "Click events discarded because the buffer was full.",
		}),
		written: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "goshort_click_events_written_total",
			Help: "Click events persisted by the worker pool.",
		}),
	}

	m.reg.MustRegister(m.redirectDuration, m.cacheLookups, m.dropped, m.written)
	m.cacheLookups.WithLabelValues("hit").Add(0)
	m.cacheLookups.WithLabelValues("miss").Add(0)
	return m
}

func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// GaugeFunc อ่านค่าตอนถูก scrape ไม่ต้องมีใครคอย set ค่าจึงไม่มีทางค้างเก่า
func (m *Metrics) TrackQueue(depth func() float64, capacity func() float64) {
	m.reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goshort_click_queue_depth",
		Help: "Events currently waiting in the click buffer.",
	}, depth))
	m.reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goshort_click_queue_capacity",
		Help: "Capacity of the click buffer.",
	}, capacity))
}

func (m *Metrics) ObserveRedirect(d time.Duration) { m.redirectDuration.Observe(d.Seconds()) }
func (m *Metrics) CacheHit()                       { m.cacheLookups.WithLabelValues("hit").Inc() }
func (m *Metrics) CacheMiss()                      { m.cacheLookups.WithLabelValues("miss").Inc() }
func (m *Metrics) SetDropped(n int64)              { setCounter(m.dropped, n) }
func (m *Metrics) SetWritten(n int64)              { setCounter(m.written, n) }

// pool นับเองด้วย atomic อยู่แล้ว ตรงนี้แค่ยกค่ามาให้ Prometheus เห็น
// Counter เพิ่มได้อย่างเดียว จึงบวกเฉพาะส่วนต่างจากค่าที่เคยรายงานไป
func setCounter(c prometheus.Counter, target int64) {
	cur := counterValue(c)
	if delta := float64(target) - cur; delta > 0 {
		c.Add(delta)
	}
}

func counterValue(c prometheus.Counter) float64 {
	var m dto.Metric
	if c.Write(&m) != nil || m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}
