package worker

import (
	"context"
	"sync"
	"time"

	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/input"
	"github.com/imrx44/redbrut/internal/output"
	"github.com/imrx44/redbrut/internal/rdp"
	"github.com/imrx44/redbrut/internal/stats"
)

// Config holds worker pool settings.
type Config struct {
	Concurrency int
	RatePerIP   float64
	Timeout     time.Duration
	LockoutPause time.Duration
	MaxRetries  int
	NoRetry     bool
}

// Pool is the goroutine pool that dispatches RDP attempts.
type Pool struct {
	cfg      Config
	limiter  *IPLimiter
	reporter *output.Reporter
	stats    *stats.Stats
	done     map[string]bool // resume state
}

func NewPool(cfg Config, reporter *output.Reporter, s *stats.Stats, done map[string]bool) *Pool {
	return &Pool{
		cfg:      cfg,
		limiter:  NewIPLimiter(cfg.RatePerIP),
		reporter: reporter,
		stats:    s,
		done:     done,
	}
}

// Run processes all jobs from the channel until it's closed or ctx is cancelled.
func (p *Pool) Run(ctx context.Context, jobs <-chan input.Job) {
	sem := make(chan struct{}, p.cfg.Concurrency)
	retryQueue := make(chan input.Job, 100000)

	var wg sync.WaitGroup

	// Retry dispatcher — feeds back into workers
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(retryQueue)
		p.dispatch(ctx, jobs, sem, retryQueue, &wg)
	}()

	// Process retries
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.dispatch(ctx, retryQueue, sem, nil, &wg)
	}()

	wg.Wait()
}

func (p *Pool) dispatch(ctx context.Context, jobs <-chan input.Job, sem chan struct{}, retryQueue chan<- input.Job, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}

			key := job.Target.String() + "\t" + job.Username + "\t" + job.Password
			if p.done[key] {
				continue
			}

			if p.limiter.IsPaused(job.Target.Host) {
				// Re-queue after pause or skip if no retry channel
				if retryQueue != nil {
					time.Sleep(100 * time.Millisecond)
					retryQueue <- job
				}
				continue
			}

			sem <- struct{}{}
			wg.Add(1)
			go func(j input.Job) {
				defer wg.Done()
				defer func() { <-sem }()
				p.attempt(ctx, j, retryQueue)
			}(job)
		}
	}
}

func (p *Pool) attempt(ctx context.Context, job input.Job, retryQueue chan<- input.Job) {
	if err := p.limiter.Wait(ctx, job.Target.Host); err != nil {
		return
	}

	result := rdp.Attempt(ctx, job, p.cfg.Timeout)
	p.stats.Attempts.Add(1)

	res := output.Result{Job: job, Status: result, Time: time.Now()}

	switch result {
	case classifier.ResultSuccess, classifier.ResultExpired:
		p.stats.Found.Add(1)
		p.reporter.WriteResult(res)

	case classifier.ResultInvalid:
		p.reporter.WriteResult(res)

	case classifier.ResultLocked:
		p.stats.Locked.Add(1)
		p.limiter.PauseIP(job.Target.Host, p.cfg.LockoutPause)
		p.reporter.WriteResult(res)

	case classifier.ResultNetworkError, classifier.ResultProtocolError, classifier.ResultUnknown:
		p.stats.Errors.Add(1)
		if !p.cfg.NoRetry && retryQueue != nil && job.Attempt < p.cfg.MaxRetries {
			job.Attempt++
			p.stats.Retrying.Add(1)
			backoff := time.Duration(1<<job.Attempt) * time.Second
			if backoff > 16*time.Second {
				backoff = 16 * time.Second
			}
			time.Sleep(backoff)
			retryQueue <- job
		}
	}
}
