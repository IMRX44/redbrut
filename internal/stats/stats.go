package stats

import "sync/atomic"

// Stats holds live counters shared between worker pool and TUI.
type Stats struct {
	Attempts atomic.Int64
	Found    atomic.Int64
	Locked   atomic.Int64
	Errors   atomic.Int64
	Retrying atomic.Int64
	Total    int64
}
