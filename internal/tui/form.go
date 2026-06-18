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
	cfg       app.Config
	goodsPath string
	resume    bool
}

type FormModel struct {
	huhForm     *huh.Form
	cfg         app.Config
	modeStr     string
	concStr     string
	rateStr     string
	timeoutStr  string
	retriesStr  string
	err         string
	width       int
	height      int
}

func NewFormModel(width, height int) FormModel {
	cfg := app.DefaultConfig()
	m := FormModel{
		cfg:        cfg,
		modeStr:    "credential",
		concStr:    "5000",
		rateStr:    "5",
		timeoutStr: "5",
		retriesStr: "3",
		width:      width,
		height:     height,
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
				Value(&m.cfg.TargetsFile),

			huh.NewInput().
				Key("users").
				Title("Users File").
				Description("One username per line").
				Placeholder("users.txt").
				Value(&m.cfg.UsersFile),

			huh.NewInput().
				Key("passwords").
				Title("Passwords File").
				Description("One password per line (UTF-8, Cyrillic/Chinese OK)").
				Placeholder("passwords.txt").
				Value(&m.cfg.PassFile),

			huh.NewInput().
				Key("output").
				Title("Output File").
				Description("Found credentials saved here").
				Placeholder("goods.txt").
				Value(&m.cfg.OutputFile),
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
				Value(&m.concStr).
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
				Value(&m.rateStr).
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
				Value(&m.timeoutStr).
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
				Value(&m.modeStr),

			huh.NewConfirm().
				Key("resume").
				Title("Resume previous session?").
				Value(&m.cfg.Resume),
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
		// Parse numeric fields
		conc, _ := strconv.Atoi(strings.TrimSpace(m.concStr))
		rate, _ := strconv.ParseFloat(strings.TrimSpace(m.rateStr), 64)
		timeout, _ := strconv.Atoi(strings.TrimSpace(m.timeoutStr))
		if conc <= 0 { conc = 5000 }
		if rate <= 0 { rate = 5 }
		if timeout <= 0 { timeout = 5 }
		if m.cfg.OutputFile == "" { m.cfg.OutputFile = "goods.txt" }

		m.cfg.Concurrency = conc
		m.cfg.RatePerIP = rate
		m.cfg.TimeoutSecs = timeout
		m.cfg.Spray = m.modeStr == "spray"

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
