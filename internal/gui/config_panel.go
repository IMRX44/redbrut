//go:build windows

package gui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func newConfigPanel(state *UIState, win fyne.Window) fyne.CanvasObject {
	// --- File inputs with Browse buttons ---
	targetsEntry := widget.NewEntry()
	targetsEntry.SetPlaceHolder("targets.txt  (IP:PORT per line)")
	state.TargetsEntry = targetsEntry

	usersEntry := widget.NewEntry()
	usersEntry.SetPlaceHolder("users.txt  (one username per line)")
	state.UsersEntry = usersEntry

	passEntry := widget.NewEntry()
	passEntry.SetPlaceHolder("passwords.txt  (UTF-8, Cyrillic/Chinese OK)")
	state.PassEntry = passEntry

	outputEntry := widget.NewEntry()
	outputEntry.SetText("goods.txt")
	state.OutputEntry = outputEntry

	makeBrowse := func(entry *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon("", browseIcon(), func() {
			dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
				if r != nil {
					entry.SetText(r.URI().Path())
				}
			}, win)
		})
	}

	targetsRow := container.NewBorder(nil, nil, nil, makeBrowse(targetsEntry), targetsEntry)
	usersRow := container.NewBorder(nil, nil, nil, makeBrowse(usersEntry), usersEntry)
	passRow := container.NewBorder(nil, nil, nil, makeBrowse(passEntry), passEntry)
	outputRow := container.NewBorder(nil, nil, nil, makeBrowse(outputEntry), outputEntry)

	// --- Settings ---
	concEntry := widget.NewEntry()
	concEntry.SetText(strconv.Itoa(state.Config.Concurrency))
	state.ConcEntry = concEntry

	rateEntry := widget.NewEntry()
	rateEntry.SetText(strconv.FormatFloat(state.Config.RatePerIP, 'f', 0, 64))
	state.RateEntry = rateEntry

	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(state.Config.TimeoutSecs))
	state.TimeoutEntry = timeoutEntry

	sprayRadio := widget.NewRadioGroup([]string{"Credential", "Spray"}, func(s string) {
		state.Config.Spray = s == "Spray"
	})
	sprayRadio.SetSelected("Credential")
	sprayRadio.Horizontal = true
	state.SprayRadio = sprayRadio

	resumeCheck := widget.NewCheck("Resume previous session", func(b bool) {
		state.Config.Resume = b
	})
	state.ResumeCheck = resumeCheck

	// --- Buttons ---
	statusLabel := widget.NewLabel("Ready")
	state.StatusLabel = statusLabel

	startBtn := widget.NewButton("  START  ", func() {
		state.startSession()
	})
	startBtn.Importance = widget.HighImportance

	stopBtn := widget.NewButton("  STOP  ", func() {
		state.stopSession()
	})
	stopBtn.Importance = widget.DangerImportance
	stopBtn.Disable()

	state.StartBtn = startBtn
	state.StopBtn = stopBtn

	btnRow := container.NewHBox(startBtn, stopBtn, statusLabel)

	// --- Form layout ---
	form := widget.NewForm(
		widget.NewFormItem("Targets File", targetsRow),
		widget.NewFormItem("Users File", usersRow),
		widget.NewFormItem("Passwords File", passRow),
		widget.NewFormItem("Output File", outputRow),
		widget.NewFormItem("", widget.NewSeparator()),
		widget.NewFormItem("Concurrency", concEntry),
		widget.NewFormItem("Rate / IP / s", rateEntry),
		widget.NewFormItem("Timeout (s)", timeoutEntry),
		widget.NewFormItem("Attack Mode", sprayRadio),
		widget.NewFormItem("", resumeCheck),
	)

	return container.NewVBox(form, widget.NewSeparator(), btnRow)
}

func browseIcon() fyne.Resource {
	return fyne.NewStaticResource("folder", []byte{})
}
