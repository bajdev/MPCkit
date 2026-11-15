package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// dialogs implements app.Dialogs with Wails' native file dialogs.
type dialogs struct {
	a *application.App
}

// cancelled maps a dialog the user closed to "", nil. On Windows Wails
// returns an error with this text instead of an empty path.
func cancelled(path string, err error) (string, error) {
	if err != nil && err.Error() == "cancelled by user" {
		return "", nil
	}
	return path, err
}

func (d *dialogs) OpenWAV() (string, error) {
	return cancelled(d.a.Dialog.OpenFile().
		SetTitle("Choose a WAV file").
		CanChooseFiles(true).
		AddFilter("WAV files", "*.wav").
		PromptForSingleSelection())
}

func (d *dialogs) OpenProject() (string, error) {
	return cancelled(d.a.Dialog.OpenFile().
		SetTitle("Load a project").
		CanChooseFiles(true).
		AddFilter("MPC Sample projects", "*.xpj").
		PromptForSingleSelection())
}

func (d *dialogs) ChooseFolder() (string, error) {
	return cancelled(d.a.Dialog.OpenFile().
		SetTitle("Export to folder").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true).
		PromptForSingleSelection())
}

func (d *dialogs) SaveZip(defaultName string) (string, error) {
	return cancelled(d.a.Dialog.SaveFile().
		SetFilename(defaultName).
		AddFilter("Zip files", "*.zip").
		PromptForSingleSelection())
}
