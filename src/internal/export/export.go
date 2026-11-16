// Package export writes a project to a folder or a zip file: the .xpj and
// its "<Name>_[ProjectData]" folder holding every pool file (build plan 4.4).
package export

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bajdev/MPCkit/internal/project"
)

// ExistsError is returned when the destination already holds the project
// and overwriting was not allowed. The caller asks the user, then retries.
type ExistsError struct {
	Paths []string
}

func (e *ExistsError) Error() string {
	return "already exists: " + strings.Join(e.Paths, ", ")
}

// MissingError lists pool files whose source can no longer be found.
type MissingError struct {
	Files []string // pool file names
	Paths []string // where each was expected
}

func (e *MissingError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d sample file(s) are missing:", len(e.Files))
	for i := range e.Files {
		fmt.Fprintf(&b, "\n  %s (%s)", e.Files[i], e.Paths[i])
	}
	return b.String()
}

type item struct {
	name string // file name inside the data folder
	src  string
}

// Usable reports whether src can be exported: a regular file, not a link,
// in a folder that is not a link either. A link in a shared kit, or a data
// folder that is itself a link, could otherwise export any file it points
// to. Links the user picks are resolved when the sample is assigned.
func Usable(src string) bool {
	st, err := os.Lstat(src)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	dir, err := os.Lstat(filepath.Dir(src))
	return err == nil && dir.IsDir()
}

// collect checks that every pool file has a usable source. A source that
// is a link or not a regular file is reported as missing.
func collect(p *project.Project) ([]item, error) {
	var items []item
	missing := &MissingError{}
	for _, f := range p.PoolFiles() {
		src, ok := p.Source(f)
		if !ok || src == "" {
			missing.Files = append(missing.Files, f)
			missing.Paths = append(missing.Paths, "no source recorded")
			continue
		}
		if !Usable(src) {
			missing.Files = append(missing.Files, f)
			missing.Paths = append(missing.Paths, src)
			continue
		}
		items = append(items, item{name: f, src: src})
	}
	if len(missing.Files) > 0 {
		return nil, missing
	}
	return items, nil
}

func encode(p *project.Project) ([]byte, error) {
	var b bytes.Buffer
	if err := p.File().Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func dataDir(p *project.Project) string { return p.Name + project.DataSuffix }

// checkNames refuses pool file names that would reach outside the data
// folder. Load and AssignSample already ensure this; it is checked again
// where files are written.
func checkNames(items []item) error {
	for _, it := range items {
		if err := project.PlainFileName(it.name); err != nil {
			return fmt.Errorf("sample %q: %w", it.name, err)
		}
	}
	return nil
}

// rename is os.Rename; tests replace it to make a rename fail.
var rename = os.Rename

// Folder writes <dest>/<Name>.xpj and <dest>/<Name>_[ProjectData]/ and
// returns the path of the .xpj. If either target exists and overwrite is
// false, nothing is written and an *ExistsError is returned.
//
// The new pair is written in full to a temporary folder in dest first. To
// replace an existing pair, Folder moves it into that temporary folder,
// moves the new pair into place, and only then deletes the old pair. If
// any step fails, the old pair is moved back, so an existing export is
// either replaced completely or left as it was. If even moving it back
// fails, the error says where the old pair was kept.
func Folder(p *project.Project, dest string, overwrite bool) (string, error) {
	if err := project.ValidName(p.Name); err != nil {
		return "", fmt.Errorf("project name: %w", err)
	}
	items, err := collect(p)
	if err != nil {
		return "", err
	}
	if err := checkNames(items); err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("export folder does not exist: %s", dest)
	}
	xpjPath := filepath.Join(dest, p.Name+".xpj")
	dirPath := filepath.Join(dest, dataDir(p))
	if !overwrite {
		if exists := existing(xpjPath, dirPath); len(exists) > 0 {
			return "", &ExistsError{Paths: exists}
		}
	}
	body, err := encode(p)
	if err != nil {
		return "", err
	}

	tmp, err := os.MkdirTemp(dest, ".mpckit-export-")
	if err != nil {
		return "", err
	}
	keepTmp := false
	defer func() {
		if !keepTmp {
			os.RemoveAll(tmp)
		}
	}()
	tmpData := filepath.Join(tmp, dataDir(p))
	if err := os.Mkdir(tmpData, 0o755); err != nil {
		return "", err
	}
	for _, it := range items {
		if err := copyFile(it.src, filepath.Join(tmpData, it.name)); err != nil {
			return "", err
		}
	}
	tmpXPJ := filepath.Join(tmp, p.Name+".xpj")
	if err := os.WriteFile(tmpXPJ, body, 0o644); err != nil {
		return "", err
	}

	// Move each existing target aside, then each new one into place. A
	// failure undoes the moves already made, newest first.
	old := filepath.Join(tmp, "old")
	if err := os.Mkdir(old, 0o755); err != nil {
		return "", err
	}
	type move struct{ from, to string }
	var moves []move
	var moveErr error
	if overwrite {
		for _, target := range existing(dirPath, xpjPath) {
			m := move{target, filepath.Join(old, filepath.Base(target))}
			if moveErr = rename(m.from, m.to); moveErr != nil {
				break
			}
			moves = append(moves, m)
		}
	}
	if moveErr == nil {
		for _, m := range []move{{tmpData, dirPath}, {tmpXPJ, xpjPath}} {
			if moveErr = rename(m.from, m.to); moveErr != nil {
				break
			}
			moves = append(moves, m)
		}
	}
	if moveErr == nil {
		return xpjPath, nil // the deferred RemoveAll deletes the old pair
	}
	for i := len(moves) - 1; i >= 0; i-- {
		if err := rename(moves[i].to, moves[i].from); err != nil {
			keepTmp = true
			return "", fmt.Errorf("%w; the previous export could not be put back and is kept in %s", moveErr, old)
		}
	}
	return "", moveErr
}

func existing(paths ...string) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Lstat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Zip writes a zip file at path holding <Name>.xpj and
// <Name>_[ProjectData]/ at its root, ready to extract onto the device's
// card. It writes to a temporary file and renames it at the end. If path
// exists and overwrite is false, an *ExistsError is returned.
func Zip(p *project.Project, path string, overwrite bool) error {
	if err := project.ValidName(p.Name); err != nil {
		return fmt.Errorf("project name: %w", err)
	}
	items, err := collect(p)
	if err != nil {
		return err
	}
	if !overwrite {
		if exists := existing(path); len(exists) > 0 {
			return &ExistsError{Paths: exists}
		}
	}
	body, err := encode(p)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".mpckit-export-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := writeZip(tmp, p, body, items); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func writeZip(w io.Writer, p *project.Project, body []byte, items []item) error {
	if err := checkNames(items); err != nil {
		return err
	}
	now := time.Now()
	zw := zip.NewWriter(w)
	// The .xpj is already gzip-compressed: store it.
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: p.Name + ".xpj", Method: zip.Store, Modified: now})
	if err != nil {
		return err
	}
	if _, err := fw.Write(body); err != nil {
		return err
	}
	dir := dataDir(p) + "/"
	if _, err := zw.CreateHeader(&zip.FileHeader{Name: dir, Modified: now}); err != nil {
		return err
	}
	for _, it := range items {
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: dir + it.name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return err
		}
		in, err := os.Open(it.src)
		if err != nil {
			return err
		}
		_, err = io.Copy(fw, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
