//go:build linux

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/imrx44/redbrut/internal/output"
)

type screenKind int

const (
	screenForm screenKind = iota
	screenMonitor
)

// RootModel is the top-level bubbletea model — owns screen switching.
type RootModel struct {
	screen  screenKind
	form    FormModel
	monitor MonitorModel
	reporter *output.Reporter
	width   int
	height  int
}

func NewRootModel() RootModel {
	return RootModel{
		screen: screenForm,
		form:   NewFormModel(80, 24),
	}
}

func (m RootModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.screen == screenForm {
			m.form.width = msg.Width
			m.form.height = msg.Height
			// Update only the huh form's width without rebuilding (which would reset inputs).
			m.form.huhForm = m.form.huhForm.WithWidth(min(msg.Width-4, 80))
		}

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.screen == screenMonitor && m.monitor.session != nil {
				m.monitor.session.Cancel()
			}
			if m.reporter != nil {
				m.reporter.Close()
			}
			return m, tea.Quit
		}

	case formSubmittedMsg:
		// Create reporter
		resumePath := msg.cfg.OutputFile + ".resume"
		reporter, err := output.NewReporter(msg.cfg.OutputFile, resumePath, false)
		if err != nil {
			// Go back to form with error
			m.form.err = err.Error()
			m.form.huhForm = m.form.buildForm()
			return m, m.form.Init()
		}
		m.reporter = reporter

		m.monitor = NewMonitorModel(m.width, m.height)
		// Wire log callback
		reporter.NotifyFn = func(res output.Result) {
			m.monitor.AddLog(res)
		}

		m.screen = screenMonitor
		return m, tea.Batch(
			m.monitor.Init(),
			startSessionCmd(msg.cfg, reporter),
		)
	}

	// Delegate to active screen
	switch m.screen {
	case screenForm:
		updated, cmd := m.form.Update(msg)
		m.form = updated.(FormModel)
		return m, cmd
	case screenMonitor:
		updated, cmd := m.monitor.Update(msg)
		m.monitor = updated.(MonitorModel)
		return m, cmd
	}
	return m, nil
}

func (m RootModel) View() string {
	switch m.screen {
	case screenForm:
		return m.form.View()
	case screenMonitor:
		return m.monitor.View()
	}
	return ""
}
