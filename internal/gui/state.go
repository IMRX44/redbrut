//go:build windows

package gui

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2/widget"
	"github.com/imrx44/redbrut/internal/app"
	"github.com/imrx44/redbrut/internal/output"
)

// UIState holds the running session and all live widgets that need updating.
type UIState struct {
	mu sync.Mutex

	Config  app.Config
	Session *app.Session
	Reporter *output.Reporter

	// Config panel widgets (set during build)
	TargetsEntry  *widget.Entry
	UsersEntry    *widget.Entry
	PassEntry     *widget.Entry
	OutputEntry   *widget.Entry
	ConcEntry     *widget.Entry
	RateEntry     *widget.Entry
	TimeoutEntry  *widget.Entry
	ResumeCheck   *widget.Check
	SprayRadio    *widget.RadioGroup
	StartBtn      *widget.Button
	StopBtn       *widget.Button
	StatusLabel   *widget.Label

	// Live panel widgets (set during build)
	LivePanel *LivePanel
}

func newUIState() *UIState {
	cfg := app.DefaultConfig()
	return &UIState{Config: cfg}
}

func (s *UIState) startSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Session != nil {
		return // already running
	}

	// Read config from widgets
	s.Config.TargetsFile = s.TargetsEntry.Text
	s.Config.UsersFile = s.UsersEntry.Text
	s.Config.PassFile = s.PassEntry.Text
	s.Config.OutputFile = s.OutputEntry.Text
	if s.Config.OutputFile == "" {
		s.Config.OutputFile = "goods.txt"
	}

	if conc, err := strconv.Atoi(s.ConcEntry.Text); err == nil && conc > 0 {
		s.Config.Concurrency = conc
	}
	if rate, err := strconv.ParseFloat(s.RateEntry.Text, 64); err == nil && rate > 0 {
		s.Config.RatePerIP = rate
	}
	if t, err := strconv.Atoi(s.TimeoutEntry.Text); err == nil && t > 0 {
		s.Config.TimeoutSecs = t
	}
	s.Config.Resume = s.ResumeCheck.Checked
	s.Config.Spray = s.SprayRadio.Selected == "Spray"

	resumePath := s.Config.OutputFile + ".resume"
	reporter, err := output.NewReporter(s.Config.OutputFile, resumePath, false)
	if err != nil {
		s.StatusLabel.SetText("Error: " + err.Error())
		return
	}
	s.Reporter = reporter

	reporter.NotifyFn = func(res output.Result) {
		s.LivePanel.AddResult(res)
	}

	session, err := app.Start(s.Config, reporter)
	if err != nil {
		reporter.Close()
		s.Reporter = nil
		s.StatusLabel.SetText("Error: " + err.Error())
		return
	}
	s.Session = session

	s.StartBtn.Disable()
	s.StopBtn.Enable()
	s.StatusLabel.SetText("Running...")
	s.LivePanel.Reset(session.Stats.Total)

	// Ticker goroutine
	go s.runTicker()

	// Watcher goroutine
	go func() {
		<-session.Done
		s.mu.Lock()
		s.Session = nil
		s.mu.Unlock()
		reporter.Close()
		s.StartBtn.Enable()
		s.StopBtn.Disable()
		s.StatusLabel.SetText(fmt.Sprintf("Done — Found: %d", session.Stats.Found.Load()))
	}()
}

func (s *UIState) stopSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Session != nil {
		s.Session.Cancel()
	}
}

func (s *UIState) runTicker() {
	lastTime := time.Now()
	lastAttempts := int64(0)

	for {
		time.Sleep(300 * time.Millisecond)

		s.mu.Lock()
		session := s.Session
		s.mu.Unlock()

		if session == nil {
			return
		}

		now := time.Now()
		elapsed := now.Sub(lastTime).Seconds()
		lastTime = now

		attempts := session.Stats.Attempts.Load()
		found := session.Stats.Found.Load()
		locked := session.Stats.Locked.Load()
		errors := session.Stats.Errors.Load()
		total := session.Stats.Total

		var speed float64
		if elapsed > 0 {
			speed = float64(attempts-lastAttempts) / elapsed
		}
		lastAttempts = attempts

		var pct float64
		if total > 0 {
			pct = float64(attempts) / float64(total)
		}

		s.LivePanel.UpdateStats(pct, speed, found, locked, errors, attempts, total)
	}
}
