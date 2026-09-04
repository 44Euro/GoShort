package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	LinkID    uint
	IPHash    string
	UserAgent string
	Referrer  string
	At        time.Time
}

type Store interface {
	WriteBatch(ctx context.Context, events []Event) error
}

type Config struct {
	Workers    int
	Buffer     int
	BatchSize  int
	FlushEvery time.Duration

	// NewTicker ให้เทสต์ยิง tick เองได้ ไม่ต้องรอเวลาจริงผ่านไป
	NewTicker func(time.Duration) (<-chan time.Time, func())
}

type Pool struct {
	ch    chan Event
	store Store
	cfg   Config

	wg      sync.WaitGroup
	closing sync.Once

	dropped atomic.Int64
	written atomic.Int64
}

func New(store Store, cfg Config) *Pool {
	if cfg.NewTicker == nil {
		cfg.NewTicker = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTicker(d)
			return t.C, t.Stop
		}
	}
	return &Pool{ch: make(chan Event, cfg.Buffer), store: store, cfg: cfg}
}

func (p *Pool) Start() {
	for i := 0; i < p.cfg.Workers; i++ {
		p.wg.Add(1)
		go p.run()
	}
}

// Enqueue ต้องไม่บล็อกเด็ดขาด คนที่รออยู่คือคนกดลิงก์ ไม่ใช่ระบบหลังบ้าน
// คิวเต็มแล้วยอมทิ้ง event ดีกว่าปล่อยให้ back-pressure วิ่งย้อนไปหาผู้ใช้
func (p *Pool) Enqueue(e Event) bool {
	select {
	case p.ch <- e:
		return true
	default:
		n := p.dropped.Add(1)
		if n == 1 || n%1000 == 0 {
			slog.Warn("click buffer full", "dropped_total", n, "capacity", cap(p.ch))
		}
		return false
	}
}

func (p *Pool) Depth() int     { return len(p.ch) }
func (p *Pool) Capacity() int  { return cap(p.ch) }
func (p *Pool) Dropped() int64 { return p.dropped.Load() }
func (p *Pool) Written() int64 { return p.written.Load() }

func (p *Pool) Shutdown(ctx context.Context) error {
	p.closing.Do(func() { close(p.ch) })

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("click writer drained", "written_total", p.written.Load())
		return nil
	case <-ctx.Done():
		slog.Error("click writer shutdown timed out", "still_queued", len(p.ch))
		return ctx.Err()
	}
}

func (p *Pool) run() {
	defer p.wg.Done()

	tick, stopTick := p.cfg.NewTicker(p.cfg.FlushEvery)
	defer stopTick()

	buf := make([]Event, 0, p.cfg.BatchSize)

	for {
		select {
		case e, ok := <-p.ch:
			if !ok {
				p.flush(buf)
				return
			}
			buf = append(buf, e)
			if len(buf) >= p.cfg.BatchSize {
				buf = p.flush(buf)
			}
		case <-tick:
			buf = p.flush(buf)
		}
	}
}

// log ตรงนี้ไม่มี request id เพราะ batch หนึ่งกินของจากหลายคำขอ ความสัมพันธ์
// หนึ่งต่อหนึ่งกับคำขอใดคำขอหนึ่งไม่มีอยู่จริงตั้งแต่ต้น
func (p *Pool) flush(buf []Event) []Event {
	if len(buf) == 0 {
		return buf
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := p.store.WriteBatch(ctx, buf); err != nil {
		slog.Error("click batch write failed", "lost_events", len(buf), "error", err)
	} else {
		p.written.Add(int64(len(buf)))
	}
	return buf[:0]
}
