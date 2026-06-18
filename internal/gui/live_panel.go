//go:build windows

package gui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/imrx44/redbrut/internal/classifier"
	"github.com/imrx44/redbrut/internal/output"
)

// LivePanel holds all widgets for the live monitoring tab.
type LivePanel struct {
	mu sync.Mutex

	progress  *widget.ProgressBar
	speedLbl  *widget.Label
	foundLbl  *widget.Label
	lockedLbl *widget.Label
	errLbl    *widget.Label
	countLbl  *widget.Label
	logList   *widget.List
	logData   []logItem
}

type logItem struct {
	text  string
	color fyne.ThemeColorName
}

func newLivePanel() (*LivePanel, fyne.CanvasObject) {
	lp := &LivePanel{
		progress:  widget.NewProgressBar(),
		speedLbl:  widget.NewLabelWithStyle("Speed: --", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		foundLbl:  widget.NewLabelWithStyle("✓ Found: 0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		lockedLbl: widget.NewLabelWithStyle("⊘ Locked: 0", fyne.TextAlignCenter, fyne.TextStyle{}),
		errLbl:    widget.NewLabelWithStyle("✗ Errors: 0", fyne.TextAlignCenter, fyne.TextStyle{}),
		countLbl:  widget.NewLabelWithStyle("0 / 0  (0.0%)", fyne.TextAlignCenter, fyne.TextStyle{}),
	}

	lp.logList = widget.NewList(
		func() int {
			lp.mu.Lock()
			defer lp.mu.Unlock()
			return len(lp.logData)
		},
		func() fyne.CanvasObject {
			return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			lp.mu.Lock()
			if id >= len(lp.logData) {
				lp.mu.Unlock()
				return
			}
			item := lp.logData[id]
			lp.mu.Unlock()
			obj.(*widget.Label).SetText(item.text)
		},
	)

	statsRow := container.NewGridWithColumns(5,
		lp.foundLbl, lp.lockedLbl, lp.errLbl, lp.speedLbl, lp.countLbl,
	)

	content := container.NewBorder(
		container.NewVBox(
			widget.NewSeparator(),
			lp.progress,
			statsRow,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		lp.logList,
	)

	return lp, content
}

func (lp *LivePanel) Reset(total int64) {
	lp.mu.Lock()
	lp.logData = nil
	lp.mu.Unlock()
	lp.progress.SetValue(0)
	lp.foundLbl.SetText("✓ Found: 0")
	lp.lockedLbl.SetText("⊘ Locked: 0")
	lp.errLbl.SetText("✗ Errors: 0")
	lp.speedLbl.SetText("Speed: --")
	lp.countLbl.SetText(fmt.Sprintf("0 / %d  (0.0%%)", total))
	lp.logList.Refresh()
}

func (lp *LivePanel) UpdateStats(pct, speed float64, found, locked, errors, attempts, total int64) {
	lp.progress.SetValue(pct)
	lp.foundLbl.SetText(fmt.Sprintf("✓ Found: %d", found))
	lp.lockedLbl.SetText(fmt.Sprintf("⊘ Locked: %d", locked))
	lp.errLbl.SetText(fmt.Sprintf("✗ Errors: %d", errors))
	lp.speedLbl.SetText(fmt.Sprintf("⚡ %.0f req/s", speed))
	lp.countLbl.SetText(fmt.Sprintf("%d / %d  (%.1f%%)", attempts, total, pct*100))
}

func (lp *LivePanel) AddResult(res output.Result) {
	var item logItem
	switch res.Status {
	case classifier.ResultSuccess, classifier.ResultExpired:
		item = logItem{
			text:  fmt.Sprintf("[+] %-24s  %-20s  %s", res.Job.Target, res.Job.Username+":"+res.Job.Password, res.Status),
			color: theme.ColorNameSuccess,
		}
	case classifier.ResultLocked:
		item = logItem{
			text:  fmt.Sprintf("[!] %-24s  %-20s  LOCKED", res.Job.Target, res.Job.Username),
			color: theme.ColorNameWarning,
		}
	default:
		return
	}

	lp.mu.Lock()
	lp.logData = append(lp.logData, item)
	if len(lp.logData) > 1000 {
		lp.logData = lp.logData[len(lp.logData)-1000:]
	}
	lp.mu.Unlock()

	lp.logList.Refresh()
	lp.logList.ScrollToBottom()
}
