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
	Concurrency  int
	RatePerIP    float64
	Timeout      time.Duration
	LockoutPause time.Duration
	MaxRetries   int
	NoRetry      bool
}

// Pool is the goroutine pool that dispatches RDP attempts.
type Pool struct {
	cfg      Config
	limiter  *IPLimiter
	reporter *output.Reporter
	stats    *stats.Stats
	done     map[string]bool
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

// Run processes all jobs until jobs channel is closed or ctx is cancelled.
func (p *Pool) Run(ctx context.Context, jobs <-chan input.Job) {
	sem := make(chan struct{}, p.cfg.Concurrency)

	// Large buffer so backoff goroutines can enqueue retries without blocking.
	retryQueue := make(chan input.Job, 500000)

	var wg sync.WaitGroup

	// Main dispatch: drains jobs, spawns workers.
	// Closes retryQueue only after all workers finish (prevents send-on-closed panic).
	wg.Add(1)
	go func() {
		defer wg.Done()
		var workerWg sync.WaitGroup
		p.dispatch(ctx, jobs, sem, retryQueue, &workerWg)
		workerWg.Wait()
		close(retryQueue)
	}()

	// Retry dispatch: drains retryQueue until closed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var workerWg sync.WaitGroup
		p.dispatch(ctx, retryQueue, sem, nil, &workerWg)
		workerWg.Wait()
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
				if retryQueue != nil {
					// Re-queue without sleeping — keep dispatch loop responsive.
					select {
					case retryQueue <- job:
					case <-ctx.Done():
						return
					}
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
			// Backoff in a lightweight goroutine — does NOT hold the semaphore slot.
			// This keeps all concurrency slots free for active connection attempts.
			go func(j input.Job, delay time.Duration) {
				select {
				case <-time.After(delay):
					p.stats.Retrying.Add(-1)
					select {
					case retryQueue <- j:
					default:
						// Buffer full — drop retry
					}
				case <-ctx.Done():
					p.stats.Retrying.Add(-1)
				}
			}(job, backoff)
		}
	}
}
