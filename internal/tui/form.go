//go:build linux

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/imrx44/redbrut/internal/app"
)

type formSubmittedMsg struct {
	cfg app.Config
}

// FormModel holds heap-allocated pointer bindings so huh and FormModel
// always share the same memory even after value-copy by bubbletea.
type FormModel struct {
	huhForm *huh.Form

	pTargets *string
	pUsers   *string
	pPass    *string
	pOutput  *string
	pConc    *string
	pRate    *string
	pTimeout *string
	pMode    *string
	pResume  *bool

	cfg    app.Config
	err    string
	width  int
	height int
}

func NewFormModel(width, height int) FormModel {
	cfg := app.DefaultConfig()

	targets := ""
	users := ""
	pass := ""
	output := cfg.OutputFile
	conc := "5000"
	rate := "5"
	timeout := "5"
	mode := "credential"
	resume := false

	m := FormModel{
		pTargets: &targets,
		pUsers:   &users,
		pPass:    &pass,
		pOutput:  &output,
		pConc:    &conc,
		pRate:    &rate,
		pTimeout: &timeout,
		pMode:    &mode,
		pResume:  &resume,
		cfg:      cfg,
		width:    width,
		height:   height,
	}
	m.huhForm = m.buildForm()
	return m
}

func (m FormModel) buildForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("  redbrut  ").
				Description("RDP Credential Testing Tool\n"),

			huh.NewInput().
				Key("targets").
				Title("Targets File").
				Description("IP:PORT per line (e.g. targets.txt)").
				Placeholder("targets.txt").
				Value(m.pTargets),

			huh.NewInput().
				Key("users").
				Title("Users File").
				Description("One username per line").
				Placeholder("users.txt").
				Value(m.pUsers),

			huh.NewInput().
				Key("passwords").
				Title("Passwords File").
				Description("One password per line (UTF-8, Cyrillic/Chinese OK)").
				Placeholder("passwords.txt").
				Value(m.pPass),

			huh.NewInput().
				Key("output").
				Title("Output File").
				Description("Found credentials saved here").
				Placeholder("goods.txt").
				Value(m.pOutput),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("  Settings  ").
				Description("Configure performance and behavior\n"),

			huh.NewInput().
				Key("concurrency").
				Title("Concurrency").
				Description("Goroutines (50–50000)").
				Placeholder("5000").
				Value(m.pConc).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 || n > 100000 {
						return fmt.Errorf("must be 1–100000")
					}
					return nil
				}),

			huh.NewInput().
				Key("rate").
				Title("Rate per IP/s").
				Description("Max attempts per IP per second").
				Placeholder("5").
				Value(m.pRate).
				Validate(func(s string) error {
					n, err := strconv.ParseFloat(s, 64)
					if err != nil || n <= 0 {
						return fmt.Errorf("must be > 0")
					}
					return nil
				}),

			huh.NewInput().
				Key("timeout").
				Title("Timeout (seconds)").
				Description("Per-attempt timeout").
				Placeholder("5").
				Value(m.pTimeout).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 {
						return fmt.Errorf("must be >= 1")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Key("mode").
				Title("Attack Mode").
				Description("Credential: all passes per target. Spray: one pass per target first").
				Options(
					huh.NewOption("Credential (all passwords per target)", "credential"),
					huh.NewOption("Password Spray (rotate passwords across targets)", "spray"),
				).
				Value(m.pMode),

			huh.NewConfirm().
				Key("resume").
				Title("Resume previous session?").
				Value(m.pResume),
		),
	).WithTheme(huh.ThemeDracula()).
		WithWidth(min(m.width-4, 80))
}

func (m FormModel) Init() tea.Cmd {
	return m.huhForm.Init()
}

func (m FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.huhForm.Update(msg)
	m.huhForm = form.(*huh.Form)

	if m.huhForm.State == huh.StateCompleted {
		conc, _ := strconv.Atoi(strings.TrimSpace(*m.pConc))
		rate, _ := strconv.ParseFloat(strings.TrimSpace(*m.pRate), 64)
		timeout, _ := strconv.Atoi(strings.TrimSpace(*m.pTimeout))
		if conc <= 0 { conc = 5000 }
		if rate <= 0 { rate = 5 }
		if timeout <= 0 { timeout = 5 }

		m.cfg.TargetsFile = strings.TrimSpace(*m.pTargets)
		m.cfg.UsersFile   = strings.TrimSpace(*m.pUsers)
		m.cfg.PassFile    = strings.TrimSpace(*m.pPass)
		m.cfg.OutputFile  = strings.TrimSpace(*m.pOutput)
		if m.cfg.OutputFile == "" { m.cfg.OutputFile = "goods.txt" }
		m.cfg.Concurrency = conc
		m.cfg.RatePerIP   = rate
		m.cfg.TimeoutSecs = timeout
		m.cfg.Spray       = *m.pMode == "spray"
		m.cfg.Resume      = *m.pResume

		return m, func() tea.Msg {
			return formSubmittedMsg{cfg: m.cfg}
		}
	}

	return m, cmd
}

func (m FormModel) View() string {
	var b strings.Builder

	b.WriteString(styleBanner.Render(banner))
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(styleError.Render("  ✗ " + m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(m.huhForm.View())

	b.WriteString("\n")
	b.WriteString(styleHint.Render("  Tab/↑↓ navigate  •  Enter confirm  •  q quit"))

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
