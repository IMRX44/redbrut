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

// MonitorModel is the live brute-force monitoring screen.
type MonitorModel struct {
	session   *app.Session
	reporter  *output.Reporter
	cfg       app.Config
	progress  progress.Model
	viewport  viewport.Model

	logCh     chan output.Result
	logLines  []string
	logMu     sync.Mutex

	startTime time.Time
	lastAttempts int64
	speed     float64

	width     int
	height    int
	done      bool
}

func NewMonitorModel(width, height int) MonitorModel {
	prog := progress.New(
		progress.WithGradient(string(colorPurple), string(colorGreen)),
		progress.WithWidth(width-8),
		progress.WithoutPercentage(),
	)
	vp := viewport.New(width-4, height-14)

	return MonitorModel{
		progress:  prog,
		viewport:  vp,
		logCh:     make(chan output.Result, 2000),
		startTime: time.Now(),
		width:     width,
		height:    height,
	}
}

func (m *MonitorModel) AddLog(res output.Result) {
	select {
	case m.logCh <- res:
	default: // never block workers
	}
}

func startSessionCmd(cfg app.Config, reporter *output.Reporter) tea.Cmd {
	return func() tea.Msg {
		session, err := app.Start(cfg, reporter)
		return sessionStartedMsg{session: session, err: err}
	}
}

func (m MonitorModel) Init() tea.Cmd {
	return tea.Batch(tickCmd())
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
		// Pass to viewport for scroll
		vp, cmd := m.viewport.Update(msg)
		m.viewport = vp
		cmds = append(cmds, cmd)

	case sessionStartedMsg:
		if msg.err != nil {
			m.done = true
			m.logLines = append(m.logLines, styleError.Render("✗ Error: "+msg.err.Error()))
			m.viewport.SetContent(strings.Join(m.logLines, "\n"))
			return m, nil
		}
		m.session = msg.session
		m.startTime = time.Now()
		cmds = append(cmds, tickCmd())

	case tickMsg:
		// Drain log channel
		draining := true
		for draining {
			select {
			case res := <-m.logCh:
				line := m.formatResult(res)
				if line != "" {
					m.logMu.Lock()
					m.logLines = append(m.logLines, line)
					if len(m.logLines) > 500 {
						m.logLines = m.logLines[len(m.logLines)-500:]
					}
					m.logMu.Unlock()
				}
			default:
				draining = false
			}
		}

		// Update viewport content
		m.logMu.Lock()
		content := strings.Join(m.logLines, "\n")
		m.logMu.Unlock()
		m.viewport.SetContent(content)
		m.viewport.GotoBottom()

		// Update speed
		if m.session != nil {
			cur := m.session.Stats.Attempts.Load()
			elapsed := time.Since(m.startTime).Seconds()
			if elapsed > 0 {
				m.speed = float64(cur) / elapsed
			}

			// Progress bar
			total := m.session.Stats.Total
			if total > 0 {
				pct := float64(cur) / float64(total)
				cmds = append(cmds, m.progress.SetPercent(pct))
			}

			// Check done
			select {
			case <-m.session.Done:
				m.done = true
				m.logLines = append(m.logLines, "")
				m.logLines = append(m.logLines, styleSuccess.Render("  ✓ Session complete"))
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
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 8
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 14
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

	// Title bar
	b.WriteString(styleHeader.Render(" redbrut  ▸  RDP Credential Testing"))
	b.WriteString("\n")

	// Progress row
	var pctStr string
	if total > 0 {
		pct := float64(attempts) / float64(total) * 100
		pctStr = fmt.Sprintf("%.1f%%", pct)
	} else {
		pctStr = "--"
	}
	b.WriteString(fmt.Sprintf("  %s  %s  /  %s  (%s)  %s\n",
		styleDim.Render("Progress:"),
		styleValue.Render(fmt.Sprintf("%d", attempts)),
		styleValue.Render(fmt.Sprintf("%d", total)),
		styleValue.Render(pctStr),
		styleDim.Render(elapsed.String()),
	))
	b.WriteString("  ")
	b.WriteString(m.progress.View())
	b.WriteString("\n\n")

	// Stats row
	b.WriteString("  ")
	b.WriteString(styleFoundStat.Render(fmt.Sprintf("✓ Found: %d", found)))
	b.WriteString(styleLockedStat.Render(fmt.Sprintf("⊘ Locked: %d", locked)))
	b.WriteString(styleErrorStat.Render(fmt.Sprintf("✗ Errors: %d", errors)))
	b.WriteString(styleRetryStat.Render(fmt.Sprintf("↻ Retry: %d", retrying)))
	b.WriteString(styleSpeedStat.Render(fmt.Sprintf("⚡ %.0f req/s", m.speed)))
	b.WriteString("\n\n")

	// Log divider
	divider := styleDim.Render("  " + strings.Repeat("─", m.width-6))
	b.WriteString(divider)
	b.WriteString("\n")

	// Viewport (scrollable log)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n")

	// Footer
	if m.done {
		b.WriteString(styleHint.Render("  Session complete  •  q to quit"))
	} else {
		b.WriteString(styleHint.Render("  ↑↓ scroll log  •  q to stop & quit"))
	}

	return b.String()
}

func (m MonitorModel) formatResult(res output.Result) string {
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
