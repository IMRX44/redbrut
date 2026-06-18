package output

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/stats"
)

var (
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	styleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleLocked  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type logEntry struct {
	text  string
	style lipgloss.Style
}

// TUI is the bubbletea model for live display.
type TUI struct {
	stats     *stats.Stats
	startTime time.Time
	lastCount int64
	speed     float64

	mu   sync.Mutex
	log  []logEntry
	done bool

	targetCount int
	workerCount int
	ratePerIP   float64
}

func NewTUI(s *stats.Stats, targetCount, workerCount int, ratePerIP float64) *TUI {
	return &TUI{
		stats:       s,
		startTime:   time.Now(),
		targetCount: targetCount,
		workerCount: workerCount,
		ratePerIP:   ratePerIP,
	}
}

func (t *TUI) AddLog(res Result) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var entry logEntry
	switch res.Status {
	case classifier.ResultSuccess, classifier.ResultExpired:
		entry.text = fmt.Sprintf("[+] %-22s  %-20s  %s", res.Job.Target, res.Job.Username+":"+res.Job.Password, res.Status)
		entry.style = styleSuccess
	case classifier.ResultLocked:
		entry.text = fmt.Sprintf("[!] %-22s  %-20s  LOCKED", res.Job.Target, res.Job.Username)
		entry.style = styleLocked
	default:
		return // don't spam TUI with errors
	}

	t.log = append(t.log, entry)
	if len(t.log) > 20 {
		t.log = t.log[len(t.log)-20:]
	}
}

func (t *TUI) SetDone() {
	t.mu.Lock()
	t.done = true
	t.mu.Unlock()
}

func (t *TUI) Init() tea.Cmd {
	return tickCmd()
}

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return t, tea.Quit
		}
	case tickMsg:
		// Calculate speed
		cur := t.stats.Attempts.Load()
		elapsed := time.Since(t.startTime).Seconds()
		if elapsed > 0 {
			t.speed = float64(cur) / elapsed
		}
		t.lastCount = cur

		t.mu.Lock()
		done := t.done
		t.mu.Unlock()
		if done {
			return t, tea.Quit
		}
		return t, tickCmd()
	}
	return t, nil
}

func (t *TUI) View() string {
	attempts := t.stats.Attempts.Load()
	found := t.stats.Found.Load()
	locked := t.stats.Locked.Load()
	errors := t.stats.Errors.Load()
	retrying := t.stats.Retrying.Load()
	total := t.stats.Total

	elapsed := time.Since(t.startTime)

	// Progress bar
	var pct float64
	var progressBar string
	if total > 0 {
		pct = float64(attempts) / float64(total) * 100
		filled := int(pct / 5)
		progressBar = "[" + strings.Repeat("█", filled) + strings.Repeat("░", 20-filled) + "]"
	} else {
		progressBar = "[" + strings.Repeat("░", 20) + "]"
	}

	// ETA
	var eta string
	if t.speed > 0 && total > 0 {
		remaining := float64(total-attempts) / t.speed
		eta = fmt.Sprintf("ETA: %s", time.Duration(remaining*float64(time.Second)).Round(time.Second))
	} else {
		eta = "ETA: --"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(styleHeader.Render(fmt.Sprintf(
		" redbrut  |  Targets: %d  Workers: %d  Rate: %.0f/IP/s",
		t.targetCount, t.workerCount, t.ratePerIP,
	)))
	sb.WriteString("\n")

	// Progress
	sb.WriteString(fmt.Sprintf(
		" %s  %d/%d (%.1f%%)  %.0f req/s  %s  [%s]\n",
		progressBar, attempts, total, pct, t.speed, elapsed.Round(time.Second), eta,
	))

	// Counters
	sb.WriteString(fmt.Sprintf(
		" %s  %s  %s  %s\n",
		styleSuccess.Render(fmt.Sprintf("Found: %d", found)),
		styleLocked.Render(fmt.Sprintf("Locked: %d", locked)),
		styleError.Render(fmt.Sprintf("Errors: %d", errors)),
		styleDim.Render(fmt.Sprintf("Retry: %d", retrying)),
	))

	sb.WriteString(styleDim.Render(strings.Repeat("─", 60)))
	sb.WriteString("\n")

	// Log entries
	t.mu.Lock()
	logs := make([]logEntry, len(t.log))
	copy(logs, t.log)
	t.mu.Unlock()

	for _, e := range logs {
		sb.WriteString(e.style.Render(e.text))
		sb.WriteString("\n")
	}

	return styleBorder.Render(sb.String())
}
