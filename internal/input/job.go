package input

import "fmt"

// Target represents a single RDP endpoint.
type Target struct {
	Host string
	Port int
}

func (t Target) String() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

// Job is one authentication attempt: target + credentials.
type Job struct {
	Target   Target
	Username string
	Password string
	Attempt  int // retry counter
}

func (j Job) String() string {
	return fmt.Sprintf("%s %s:%s", j.Target, j.Username, j.Password)
}
