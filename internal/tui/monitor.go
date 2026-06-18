//go:build linux

package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/imrx44/redbrut/internal/app"
	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/output"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type sessionStartedMsg struct {
	session *app.Session
	err     error
}

// logBuf is heap-allocated so MonitorModel can be copied without copying a mutex.
type logBuf struct {
	mu    sync.Mutex
	lines []string
	ch    chan output.Result
}

func newLogBuf() *logBuf {
	return &logBuf{ch: make(chan output.Result, 2000)}
}

func (lb *logBuf) push(line string) {
	lb.mu.Lock()
	lb.lines = append(lb.lines, line)
	if len(lb.lines) > 500 {
		lb.lines = lb.lines[len(lb.lines)-500:]
	}
	lb.mu.Unlock()
}

func (lb *logBuf) snapshot() string {
	lb.mu.Lock()
	s := strings.Join(lb.lines, "\n")
	lb.mu.Unlock()
	return s
}

// MonitorModel is the live brute-force monitoring screen.
// All mutable shared state is behind pointers so value-copy is safe.
type MonitorModel struct {
	session  *app.Session
	log      *logBuf
	progress progress.Model
	viewport viewport.Model

	startTime time.Time
	speed     float64

	width  int
	height int
	done   bool
}

func NewMonitorModel(width, height int) MonitorModel {
	vph := height - 14
	if vph < 4 {
		vph = 4
	}
	prog := progress.New(
		progress.WithGradient(string(colorPurple), string(colorGreen)),
		progress.WithWidth(width-8),
		progress.WithoutPercentage(),
	)
	return MonitorModel{
		log:       newLogBuf(),
		progress:  prog,
		viewport:  viewport.New(width-4, vph),
		startTime: time.Now(),
		width:     width,
		height:    height,
	}
}

// AddLog is called from worker goroutines — sends result to channel, never blocks.
func (m *MonitorModel) AddLog(res output.Result) {
	select {
	case m.log.ch <- res:
	default:
	}
}

func startSessionCmd(cfg app.Config, reporter *output.Reporter) tea.Cmd {
	return func() tea.Msg {
		session, err := app.Start(cfg, reporter)
		return sessionStartedMsg{session: session, err: err}
	}
}

func (m MonitorModel) Init() tea.Cmd {
	return tickCmd()
}

func (m MonitorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.session != nil {
				m.session.Cancel()
			}
			return m, tea.Quit
		}

	case sessionStartedMsg:
		if msg.err != nil {
			m.done = true
			m.log.push(styleError.Render("  ✗ Error: " + msg.err.Error()))
			m.viewport.SetContent(m.log.snapshot())
			return m, nil
		}
		m.session = msg.session
		m.startTime = time.Now()
		cmds = append(cmds, tickCmd())

	case tickMsg:
		// Drain log channel into buffer
		for {
			select {
			case res := <-m.log.ch:
				if line := formatResult(res); line != "" {
					m.log.push(line)
				}
			default:
				goto drained
			}
		}
	drained:
		m.viewport.SetContent(m.log.snapshot())
		m.viewport.GotoBottom()

		if m.session != nil {
			cur := m.session.Stats.Attempts.Load()
			if elapsed := time.Since(m.startTime).Seconds(); elapsed > 0 {
				m.speed = float64(cur) / elapsed
			}
			if total := m.session.Stats.Total; total > 0 {
				cmds = append(cmds, m.progress.SetPercent(float64(cur)/float64(total)))
			}
			select {
			case <-m.session.Done:
				m.done = true
				m.log.push("")
				m.log.push(styleSuccess.Render("  ✓ Session complete"))
				m.viewport.SetContent(m.log.snapshot())
				m.viewport.GotoBottom()
				return m, nil
			default:
			}
		}
		cmds = append(cmds, tickCmd())

	case progress.FrameMsg:
		prog, cmd := m.progress.Update(msg)
		m.progress = prog.(progress.Model)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.Width = msg.Width - 8
		m.viewport.Width = msg.Width - 4
		vph := msg.Height - 14
		if vph < 4 {
			vph = 4
		}
		m.viewport.Height = vph
	}

	vp, cmd := m.viewport.Update(msg)
	m.viewport = vp
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m MonitorModel) View() string {
	if m.session == nil {
		return styleHint.Render("  Initializing...")
	}

	attempts := m.session.Stats.Attempts.Load()
	found := m.session.Stats.Found.Load()
	locked := m.session.Stats.Locked.Load()
	errors := m.session.Stats.Errors.Load()
	retrying := m.session.Stats.Retrying.Load()
	total := m.session.Stats.Total
	elapsed := time.Since(m.startTime).Round(time.Second)

	var b strings.Builder

	b.WriteString(styleHeader.Render(" redbrut  ▸  RDP Credential Testing"))
	b.WriteString("\n")

	var pctStr string
	if total > 0 {
		pctStr = fmt.Sprintf("%.1f%%", float64(attempts)/float64(total)*100)
	} else {
		pctStr = "--"
	}
	b.WriteString(fmt.Sprintf("  %s  %s / %s  (%s)  elapsed: %s\n",
		styleDim.Render("Progress:"),
		styleValue.Render(fmt.Sprintf("%d", attempts)),
		styleValue.Render(fmt.Sprintf("%d", total)),
		styleValue.Render(pctStr),
		styleDim.Render(elapsed.String()),
	))
	b.WriteString("  ")
	b.WriteString(m.progress.View())
	b.WriteString("\n\n")

	b.WriteString("  ")
	b.WriteString(styleFoundStat.Render(fmt.Sprintf("✓ Found: %d", found)))
	b.WriteString(styleLockedStat.Render(fmt.Sprintf("  ⊘ Locked: %d", locked)))
	b.WriteString(styleErrorStat.Render(fmt.Sprintf("  ✗ Errors: %d", errors)))
	b.WriteString(styleRetryStat.Render(fmt.Sprintf("  ↻ Retry: %d", retrying)))
	b.WriteString(styleSpeedStat.Render(fmt.Sprintf("  ⚡ %.0f req/s", m.speed)))
	b.WriteString("\n\n")

	divider := styleDim.Render("  " + strings.Repeat("─", m.width-6))
	b.WriteString(divider)
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n")

	if m.done {
		b.WriteString(styleHint.Render("  Session complete  •  q to quit"))
	} else {
		b.WriteString(styleHint.Render("  ↑↓ scroll log  •  q to stop & quit"))
	}
	return b.String()
}

func formatResult(res output.Result) string {
	switch res.Status {
	case classifier.ResultSuccess, classifier.ResultExpired:
		return styleSuccess.Render(fmt.Sprintf(
			"  [+] %-24s  %-20s  %s",
			res.Job.Target, res.Job.Username+":"+res.Job.Password, res.Status,
		))
	case classifier.ResultLocked:
		return styleLocked.Render(fmt.Sprintf(
			"  [!] %-24s  %-20s  LOCKED",
			res.Job.Target, res.Job.Username,
		))
	}
	return ""
}
