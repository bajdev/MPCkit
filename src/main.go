// Command mpckit is MPCkit, the desktop kit editor for the MPC Sample.
package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/bajdev/MPCkit/internal/app"
	"github.com/bajdev/MPCkit/internal/project"
)

// The built Svelte app (frontend/dist), embedded in the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

// Window sizes are in device-independent pixels, 16:9 like most displays.
const (
	maxWidth, maxHeight = 1280, 720 // largest default size
	minWidth, minHeight = 960, 540  // smallest the layout is designed for
	screenShare         = 0.9       // default size: this share of the screen's work area
)

// Events Go sends to the frontend.
const (
	eventState  = "state"  // a new app.State after a change the frontend did not ask for
	eventError  = "error"  // a message to show
	eventClose  = "close"  // the window was asked to close with unsaved changes
	eventNotice = "notice" // a short success message, e.g. what a drop assigned
	eventBank   = "bank"   // show this bank (0 for A): files were dropped on it
)

func init() {
	application.RegisterEvent[app.State](eventState)
	application.RegisterEvent[string](eventError)
	application.RegisterEvent[application.Void](eventClose)
	application.RegisterEvent[string](eventNotice)
	application.RegisterEvent[int](eventBank)
}

func main() {
	d := &dialogs{}
	svc, host := app.NewService(d)
	a := newApp(svc, host)
	d.a = a
	w := newWindow(a)
	guardClose(a, w, host)
	handleFileDrops(a, w, host)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// newApp creates the application. Only svc is bound; host serves the
// sample previews.
func newApp(svc *app.Service, host *app.Host) *application.App {
	var a *application.App
	a = application.New(application.Options{
		Name:        "MPCkit",
		Description: "Desktop kit editor for the MPC Sample",
		// Wails exits on a panic in an event handler unless this is set.
		// Log it and tell the user instead, so a case the checks missed
		// cannot close the app and lose their changes.
		PanicHandler: func(p *application.PanicDetails) {
			log.Printf("panic: %v\n%s", p.Error, p.StackTrace)
			a.Event.Emit(eventError, "Something went wrong inside MPCkit, and the last action may not have finished.")
		},
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: host.AudioMiddleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	return a
}

// newWindow opens the main window. It starts hidden and is sized once the
// screens are known, so it fits the display instead of being clamped by it.
func newWindow(a *application.App) *application.WebviewWindow {
	w := a.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "MPCkit",
		Width:            maxWidth,
		Height:           maxHeight,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		Hidden:           true,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(0x15, 0x17, 0x1A),
		URL:              "/",
	})
	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if s := a.Screen.GetPrimary(); s != nil {
			width, height := fitSize(s.WorkArea.Width, s.WorkArea.Height)
			w.SetSize(width, height)
		}
		w.Center()
		w.Show()
	})
	return w
}

// guardClose handles closing with unsaved changes: stop, and let the
// frontend ask. If the user chooses to discard them, the frontend calls
// CloseWindow, which closes for real.
func guardClose(a *application.App, w *application.WebviewWindow, host *app.Host) {
	var closeConfirmed atomic.Bool
	host.SetCloser(func() {
		closeConfirmed.Store(true)
		w.Close()
	})
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if closeConfirmed.Load() || !host.Dirty() {
			return
		}
		e.Cancel()
		a.Event.Emit(eventClose)
	})
}

// handleFileDrops handles files dropped on the window's drop targets:
//   - several files or any folder dropped on a bank letter (an element with
//     data-bank) or on a pad (data-slot) fill that bank, if it is empty
//     (docs/multi-drop-plan.md);
//   - one WAV file dropped on a pad is assigned to that pad, replacing what
//     is there. A single file dropped on a bank letter fills the bank too.
func handleFileDrops(a *application.App, w *application.WebviewWindow, host *app.Host) {
	w.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		ctx := e.Context()
		target := ctx.DropTargetDetails()
		if target == nil {
			return
		}
		files := ctx.DroppedFiles()
		if bank, err := strconv.Atoi(target.Attributes["data-bank"]); err == nil {
			dropOnBank(a, host, bank, files)
			return
		}
		slot, err := strconv.Atoi(target.Attributes["data-slot"])
		if err != nil {
			return
		}
		if len(files) > 1 || anyFolder(files) {
			dropOnBank(a, host, slot/project.PadsPerBank, files)
			return
		}
		for _, f := range files {
			if strings.EqualFold(filepath.Ext(f), ".wav") {
				st, err := host.AssignSamplePath(slot, f)
				a.Event.Emit(eventState, st)
				if err != nil {
					a.Event.Emit(eventError, err.Error())
				}
				return
			}
		}
		a.Event.Emit(eventError, app.NoWAVs)
	})
}

// dropOnBank fills bank from the dropped paths. The window shows that bank
// first, also when the drop is refused, so the reason is in view.
func dropOnBank(a *application.App, host *app.Host, bank int, files []string) {
	a.Event.Emit(eventBank, bank)
	st, notice, err := host.DropOnBank(bank, files)
	a.Event.Emit(eventState, st)
	if notice != "" {
		a.Event.Emit(eventNotice, notice)
	}
	if err != nil {
		a.Event.Emit(eventError, err.Error())
	}
}

func anyFolder(paths []string) bool {
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// fitSize returns a 16:9 window size that takes screenShare of the work
// area (areaW x areaH), capped at maxWidth x maxHeight and no smaller than
// minWidth x minHeight.
func fitSize(areaW, areaH int) (int, int) {
	width := min(maxWidth, int(float64(areaW)*screenShare))
	height := width * 9 / 16
	if limit := int(float64(areaH) * screenShare); height > limit {
		height = limit
		width = height * 16 / 9
	}
	return max(width, minWidth), max(height, minHeight)
}
