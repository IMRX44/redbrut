package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/input"
)

// Result is a completed authentication attempt.
type Result struct {
	Job    input.Job
	Status classifier.AuthResult
	Time   time.Time
}

// Reporter writes results to goods.txt and resume.state.
type Reporter struct {
	mu         sync.Mutex
	goodsFile  *os.File
	resumeFile *os.File
	jsonMode   bool
	NotifyFn   func(Result) // called on every result; must be non-blocking
}

func NewReporter(goodsPath, resumePath string, jsonMode bool) (*Reporter, error) {
	gf, err := os.OpenFile(goodsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open goods file: %w", err)
	}
	rf, err := os.OpenFile(resumePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		gf.Close()
		return nil, fmt.Errorf("open resume file: %w", err)
	}
	return &Reporter{goodsFile: gf, resumeFile: rf, jsonMode: jsonMode}, nil
}

func (r *Reporter) Close() {
	r.goodsFile.Close()
	r.resumeFile.Close()
}

// WriteResult records a completed job.
func (r *Reporter) WriteResult(res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fmt.Fprintf(r.resumeFile, "%s\t%s\t%s\t%s\n",
		res.Job.Target, res.Job.Username, res.Job.Password, res.Status)

	if res.Status.IsSuccess() {
		if r.jsonMode {
			entry := map[string]string{
				"target":   res.Job.Target.String(),
				"username": res.Job.Username,
				"password": res.Job.Password,
				"status":   res.Status.String(),
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(r.goodsFile, "%s\n", data)
		} else {
			fmt.Fprintf(r.goodsFile, "%s\t%s\t%s\n",
				res.Job.Target, res.Job.Username, res.Job.Password)
		}
	}

	if r.NotifyFn != nil {
		r.NotifyFn(res)
	}
}

// LoadResumeState reads resume.state and returns a set of already-completed job keys.
func LoadResumeState(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return make(map[string]bool), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	done := make(map[string]bool)
	var target, user, pass, status string
	for {
		n, _ := fmt.Fscanf(f, "%s\t%s\t%s\t%s\n", &target, &user, &pass, &status)
		if n == 0 {
			break
		}
		done[target+"\t"+user+"\t"+pass] = true
	}
	return done, nil
}
