package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goshort/internal/worker"
)

type recordingStore struct {
	mu      sync.Mutex
	batches [][]worker.Event
	writes  chan int
}

func newStore() *recordingStore {
	return &recordingStore{writes: make(chan int, 64)}
}

func (s *recordingStore) WriteBatch(_ context.Context, events []worker.Event) error {
	s.mu.Lock()
	batch := make([]worker.Event, len(events))
	copy(batch, events)
	s.batches = append(s.batches, batch)
	s.mu.Unlock()
	s.writes <- len(events)
	return nil
}

func (s *recordingStore) snapshot() [][]worker.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]worker.Event, len(s.batches))
	copy(out, s.batches)
	return out
}

func (s *recordingStore) total() int {
	n := 0
	for _, b := range s.snapshot() {
		n += len(b)
	}
	return n
}

func (s *recordingStore) awaitWrite(t *testing.T) int {
	t.Helper()
	select {
	case n := <-s.writes:
		return n
	case <-time.After(2 * time.Second):
		t.Fatal("expected a batch write, none arrived")
		return 0
	}
}

type manualTicker struct{ c chan time.Time }

func (m *manualTicker) new(time.Duration) (<-chan time.Time, func()) { return m.c, func() {} }
func (m *manualTicker) tick()                                        { m.c <- time.Now() }

func poolWith(store worker.Store, workers, batch int, tick *manualTicker) *worker.Pool {
	cfg := worker.Config{Workers: workers, Buffer: 100, BatchSize: batch, FlushEvery: time.Hour}
	if tick != nil {
		cfg.NewTicker = tick.new
	}
	return worker.New(store, cfg)
}

func TestAFullBatchIsWrittenInOneGo(t *testing.T) {
	store := newStore()
	p := poolWith(store, 1, 5, nil)
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	for i := 0; i < 5; i++ {
		require.True(t, p.Enqueue(worker.Event{LinkID: 1}))
	}

	require.Equal(t, 5, store.awaitWrite(t), "five events should arrive as one batch, not five writes")
}

func TestAPartialBatchIsFlushedOnTheTimer(t *testing.T) {
	store := newStore()
	tick := &manualTicker{c: make(chan time.Time)}
	p := poolWith(store, 1, 50, tick)
	p.Start()
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	require.True(t, p.Enqueue(worker.Event{LinkID: 1}))
	require.True(t, p.Enqueue(worker.Event{LinkID: 1}))

	// รอให้ worker ดูดออกจาก channel จนหมดก่อน ไม่งั้น tick อาจแทรกกลางแล้ว
	// flush ทีละตัว ซึ่งไม่ใช่สิ่งที่เทสต์นี้ต้องการวัด
	require.Eventually(t, func() bool { return p.Depth() == 0 }, time.Second, time.Millisecond)

	tick.tick()

	require.Equal(t, 2, store.awaitWrite(t), "the timer should flush what is buffered even below batch size")
	require.Equal(t, int64(2), p.Written(), "the flush must come from the timer, not from shutdown")
}

func TestEnqueueNeverBlocksWhenTheBufferIsFull(t *testing.T) {
	store := newStore()
	// ไม่ Start() เลย ไม่มีใครดึงออก คิวจึงเต็มแน่นอน
	p := worker.New(store, worker.Config{Workers: 1, Buffer: 3, BatchSize: 10, FlushEvery: time.Hour})

	for i := 0; i < 3; i++ {
		require.True(t, p.Enqueue(worker.Event{LinkID: 1}))
	}

	done := make(chan bool, 1)
	go func() { done <- p.Enqueue(worker.Event{LinkID: 1}) }()

	select {
	case accepted := <-done:
		require.False(t, accepted, "a full buffer must reject, not accept")
	case <-time.After(time.Second):
		t.Fatal("Enqueue blocked on a full buffer; the redirect path would have stalled")
	}

	require.Equal(t, int64(1), p.Dropped())
}

func TestShutdownDrainsWhatIsStillQueued(t *testing.T) {
	store := newStore()
	p := poolWith(store, 2, 1000, nil)
	p.Start()

	const n = 40
	for i := 0; i < n; i++ {
		require.True(t, p.Enqueue(worker.Event{LinkID: uint(i%3 + 1)}))
	}

	require.NoError(t, p.Shutdown(context.Background()))

	require.Equal(t, n, store.total(), "shutdown must write everything still in the queue")
	require.Equal(t, int64(n), p.Written())
}

func TestDepthAndCapacityAreObservable(t *testing.T) {
	store := newStore()
	p := worker.New(store, worker.Config{Workers: 1, Buffer: 10, BatchSize: 10, FlushEvery: time.Hour})

	p.Enqueue(worker.Event{LinkID: 1})
	p.Enqueue(worker.Event{LinkID: 1})

	require.Equal(t, 2, p.Depth())
	require.Equal(t, 10, p.Capacity())
}
