package rdp

import (
	"context"
	"time"

	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/input"
)

// Attempt performs a single RDP NLA authentication attempt for the given job.
func Attempt(ctx context.Context, job input.Job, timeout time.Duration) classifier.AuthResult {
	return AttemptNLA(ctx, job.Target.Host, job.Target.Port, job.Username, job.Password, timeout)
}
