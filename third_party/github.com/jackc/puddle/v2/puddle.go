package puddle

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrClosedPool    = errors.New("puddle: pool closed")
	ErrNotAvailable  = errors.New("puddle: resource not available")
	errNoConstructor = errors.New("puddle: constructor required")
)

type Config[T any] struct {
	Constructor func(context.Context) (T, error)
	Destructor  func(T)
	MaxSize     int32
}

type pooledResource[T any] struct {
	value         T
	idleSince     time.Time
	shouldDestroy bool
	hijacked      bool
}

type Pool[T any] struct {
	mu       sync.Mutex
	cfg      *Config[T]
	closed   bool
	idle     []*pooledResource[T]
	acquired map[*pooledResource[T]]struct{}
	stats    statData
}

type statData struct {
	acquireCount          int64
	acquireDuration       time.Duration
	canceledAcquireCount  int64
	emptyAcquireCount     int64
	constructingResources int32
	maxResources          int32
}

type Resource[T any] struct {
	pool *Pool[T]
	res  *pooledResource[T]
}

func NewPool[T any](cfg *Config[T]) (*Pool[T], error) {
	if cfg == nil || cfg.Constructor == nil {
		return nil, errNoConstructor
	}

	p := &Pool[T]{
		cfg:      cfg,
		idle:     make([]*pooledResource[T], 0),
		acquired: make(map[*pooledResource[T]]struct{}),
		stats: statData{
			maxResources: cfg.MaxSize,
		},
	}
	return p, nil
}

func (p *Pool[T]) Acquire(ctx context.Context) (*Resource[T], error) {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			p.recordCanceledAcquire()
			return nil, ctx.Err()
		default:
		}

		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, ErrClosedPool
		}
		if len(p.idle) > 0 {
			idx := len(p.idle) - 1
			pr := p.idle[idx]
			p.idle[idx] = nil
			p.idle = p.idle[:idx]
			pr.idleSince = time.Time{}
			p.acquired[pr] = struct{}{}
			p.mu.Unlock()
			p.recordAcquire(time.Since(start))
			return &Resource[T]{pool: p, res: pr}, nil
		}
		maxSize := int(p.cfg.MaxSize)
		total := len(p.idle) + len(p.acquired)
		if maxSize > 0 && total >= maxSize {
			p.stats.emptyAcquireCount++
			p.mu.Unlock()
			return nil, ErrNotAvailable
		}
		p.stats.constructingResources++
		p.mu.Unlock()

		val, err := p.cfg.Constructor(ctx)

		p.mu.Lock()
		p.stats.constructingResources--
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				p.stats.canceledAcquireCount++
			}
			p.mu.Unlock()
			return nil, err
		}
		pr := &pooledResource[T]{value: val}
		p.acquired[pr] = struct{}{}
		p.mu.Unlock()
		p.recordAcquire(time.Since(start))
		return &Resource[T]{pool: p, res: pr}, nil
	}
}

func (p *Pool[T]) AcquireAllIdle() []*Resource[T] {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || len(p.idle) == 0 {
		return nil
	}
	res := make([]*Resource[T], 0, len(p.idle))
	for len(p.idle) > 0 {
		idx := len(p.idle) - 1
		pr := p.idle[idx]
		p.idle[idx] = nil
		p.idle = p.idle[:idx]
		p.acquired[pr] = struct{}{}
		res = append(res, &Resource[T]{pool: p, res: pr})
	}
	return res
}

func (p *Pool[T]) CreateResource(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrClosedPool
	}
	maxSize := int(p.cfg.MaxSize)
	total := len(p.idle) + len(p.acquired)
	if maxSize > 0 && total >= maxSize {
		p.stats.emptyAcquireCount++
		p.mu.Unlock()
		return ErrNotAvailable
	}
	p.stats.constructingResources++
	p.mu.Unlock()

	val, err := p.cfg.Constructor(ctx)

	p.mu.Lock()
	p.stats.constructingResources--
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			p.stats.canceledAcquireCount++
		}
		p.mu.Unlock()
		return err
	}
	if p.closed {
		p.mu.Unlock()
		p.callDestructor(val)
		return ErrClosedPool
	}
	pr := &pooledResource[T]{value: val, idleSince: time.Now()}
	p.idle = append(p.idle, pr)
	p.mu.Unlock()
	return nil
}

func (p *Pool[T]) Reset() {
	p.mu.Lock()
	idle := p.idle
	p.idle = nil
	for pr := range p.acquired {
		pr.shouldDestroy = true
	}
	p.mu.Unlock()

	for _, pr := range idle {
		p.callDestructor(pr.value)
	}
}

func (p *Pool[T]) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	idle := p.idle
	p.idle = nil
	acquired := p.acquired
	p.acquired = make(map[*pooledResource[T]]struct{})
	p.mu.Unlock()

	for _, pr := range idle {
		p.callDestructor(pr.value)
	}
	for pr := range acquired {
		p.callDestructor(pr.value)
	}
}

func (p *Pool[T]) Stat() *Stat {
	p.mu.Lock()
	defer p.mu.Unlock()
	stat := &Stat{
		acquireCount:          p.stats.acquireCount,
		acquireDuration:       p.stats.acquireDuration,
		canceledAcquireCount:  p.stats.canceledAcquireCount,
		emptyAcquireCount:     p.stats.emptyAcquireCount,
		constructingResources: p.stats.constructingResources,
		idleResources:         int32(len(p.idle)),
		totalResources:        int32(len(p.idle) + len(p.acquired)),
		maxResources:          p.stats.maxResources,
		acquiredResources:     int32(len(p.acquired)),
	}
	return stat
}

func (p *Pool[T]) recordAcquire(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.acquireCount++
	p.stats.acquireDuration += d
}

func (p *Pool[T]) recordCanceledAcquire() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.canceledAcquireCount++
}

func (p *Pool[T]) callDestructor(value T) {
	if p.cfg.Destructor == nil {
		return
	}
	if isNil(value) {
		return
	}
	p.cfg.Destructor(value)
}

func (r *Resource[T]) Value() T {
	var zero T
	if r == nil || r.res == nil {
		return zero
	}
	return r.res.value
}

func (r *Resource[T]) Release() {
	if r == nil || r.pool == nil || r.res == nil {
		return
	}
	p := r.pool
	pr := r.res
	r.res = nil
	r.pool = nil

	p.mu.Lock()
	if _, ok := p.acquired[pr]; !ok {
		p.mu.Unlock()
		return
	}
	delete(p.acquired, pr)
	if p.closed || pr.shouldDestroy {
		p.mu.Unlock()
		p.callDestructor(pr.value)
		return
	}
	pr.idleSince = time.Now()
	p.idle = append(p.idle, pr)
	p.mu.Unlock()
}

func (r *Resource[T]) ReleaseUnused() {
	r.Release()
}

func (r *Resource[T]) Destroy() {
	if r == nil || r.pool == nil || r.res == nil {
		return
	}
	p := r.pool
	pr := r.res
	r.res = nil
	r.pool = nil

	p.mu.Lock()
	if _, ok := p.acquired[pr]; ok {
		delete(p.acquired, pr)
		p.mu.Unlock()
		p.callDestructor(pr.value)
		return
	}
	for i, candidate := range p.idle {
		if candidate == pr {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	p.callDestructor(pr.value)
}

func (r *Resource[T]) Hijack() {
	if r == nil || r.pool == nil || r.res == nil {
		return
	}
	p := r.pool
	pr := r.res
	r.res = nil
	r.pool = nil

	p.mu.Lock()
	delete(p.acquired, pr)
	p.mu.Unlock()
}

func (r *Resource[T]) IdleDuration() time.Duration {
	if r == nil || r.res == nil {
		return 0
	}
	if r.res.idleSince.IsZero() {
		return 0
	}
	return time.Since(r.res.idleSince)
}

func isNil[T any](v T) bool {
	return any(v) == nil
}

type Stat struct {
	acquireCount          int64
	acquireDuration       time.Duration
	canceledAcquireCount  int64
	emptyAcquireCount     int64
	constructingResources int32
	idleResources         int32
	totalResources        int32
	maxResources          int32
	acquiredResources     int32
}

func (s *Stat) AcquireCount() int64 { return s.acquireCount }

func (s *Stat) AcquireDuration() time.Duration { return s.acquireDuration }

func (s *Stat) AcquiredResources() int32 { return s.acquiredResources }

func (s *Stat) CanceledAcquireCount() int64 { return s.canceledAcquireCount }

func (s *Stat) ConstructingResources() int32 { return s.constructingResources }

func (s *Stat) EmptyAcquireCount() int64 { return s.emptyAcquireCount }

func (s *Stat) IdleResources() int32 { return s.idleResources }

func (s *Stat) MaxResources() int32 { return s.maxResources }

func (s *Stat) TotalResources() int32 { return s.totalResources }
