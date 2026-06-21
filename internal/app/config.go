package app

import (
	"context"
	"fmt"
	"time"

	"github.com/imrx44/redbrut/internal/input"
	"github.com/imrx44/redbrut/internal/output"
	"github.com/imrx44/redbrut/internal/stats"
	"github.com/imrx44/redbrut/internal/worker"
)

// Config is the canonical settings struct both UIs populate before starting.
type Config struct {
	TargetsFile string
	SingleHost  string
	UsersFile   string
	PassFile    string
	OutputFile  string
	Concurrency int
	RatePerIP   float64
	TimeoutSecs int
	Spray       bool
	Resume      bool
	JSONMode    bool
	NoRetry     bool
	MaxRetries  int
}

func DefaultConfig() Config {
	return Config{
		OutputFile:  "goods.txt",
		Concurrency: 5000,
		RatePerIP:   5,
		TimeoutSecs: 5,
		MaxRetries:  3,
	}
}

// Session is a live brute-force session. Both UIs hold a *Session.
type Session struct {
	Stats  *stats.Stats
	Cancel context.CancelFunc
	Done   <-chan struct{} // closed when pool.Run() returns
}

// Start validates config, loads files, and launches the worker pool.
// logFn receives notable results (success, locked) in real time — must be non-blocking.
func Start(cfg Config, reporter *output.Reporter) (*Session, error) {
	var targets []input.Target
	if cfg.SingleHost != "" {
		t, err := input.ParseTarget(cfg.SingleHost)
		if err != nil {
			return nil, fmt.Errorf("parse host: %w", err)
		}
		targets = []input.Target{t}
	} else {
		var err error
		targets, err = input.LoadTargets(cfg.TargetsFile)
		if err != nil {
			return nil, fmt.Errorf("load targets: %w", err)
		}
	}

	users, err := input.LoadLines(cfg.UsersFile)
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}

	passwords, err := input.LoadLines(cfg.PassFile)
	if err != nil {
		return nil, fmt.Errorf("load passwords: %w", err)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("targets list is empty")
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("users list is empty")
	}
	if len(passwords) == 0 {
		return nil, fmt.Errorf("passwords list is empty")
	}

	resumePath := cfg.OutputFile + ".resume"
	var doneJobs map[string]bool
	if cfg.Resume {
		doneJobs, err = output.LoadResumeState(resumePath)
		if err != nil {
			return nil, fmt.Errorf("load resume state: %w", err)
		}
	} else {
		doneJobs = make(map[string]bool)
	}

	s := &stats.Stats{}
	s.Total = int64(len(targets))*int64(len(users))*int64(len(passwords)) - int64(len(doneJobs))

	pool := worker.NewPool(worker.Config{
		Concurrency:  cfg.Concurrency,
		RatePerIP:    cfg.RatePerIP,
		Timeout:      time.Duration(cfg.TimeoutSecs) * time.Second,
		LockoutPause: 30 * time.Minute,
		MaxRetries:   cfg.MaxRetries,
		NoRetry:      cfg.NoRetry,
	}, reporter, s, doneJobs)

	mode := input.ModeCredential
	if cfg.Spray {
		mode = input.ModeSpray
	}
	jobs := input.GenerateCombos(targets, users, passwords, mode)

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		pool.Run(ctx, jobs)
	}()

	return &Session{
		Stats:  s,
		Cancel: cancel,
		Done:   doneCh,
	}, nil
}
