// Command mpckit is MPCkit, the desktop kit editor for the MPC Sample.
package main

import (
	"embed"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"git.monoexp.com/bajdev/mpckit/internal/app"
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
	eventState = "state" // a new app.State after a change the frontend did not ask for
	eventError = "error" // a message to show
	eventClose = "close" // the window was asked to close with unsaved changes
)

func init() {
	application.RegisterEvent[app.State](eventState)
	application.RegisterEvent[string](eventError)
	application.RegisterEvent[application.Void](eventClose)
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

// handleFileDrops assigns a WAV dropped on a pad (an element with
// data-file-drop-target and data-slot) to that pad.
func handleFileDrops(a *application.App, w *application.WebviewWindow, host *app.Host) {
	w.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		ctx := e.Context()
		target := ctx.DropTargetDetails()
		if target == nil {
			return
		}
		slot, err := strconv.Atoi(target.Attributes["data-slot"])
		if err != nil {
			return
		}
		for _, f := range ctx.DroppedFiles() {
			if strings.EqualFold(filepath.Ext(f), ".wav") {
				st, err := host.AssignSamplePath(slot, f)
				a.Event.Emit(eventState, st)
				if err != nil {
					a.Event.Emit(eventError, err.Error())
				}
				return
			}
		}
		a.Event.Emit(eventError, "Only WAV files can be assigned to a pad.")
	})
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
