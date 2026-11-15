// Command xpjdiff compares two MPC Sample projects: the decompressed .xpj
// files and the sample files in their data folders. It prints the paths of
// the values that differ, or "identical". Works the same in cmd,
// PowerShell and sh:
//
//	go run ./tools/xpjdiff A.xpj B.xpj
//
// The project name is not part of the comparison (it lives only in the file
// and folder names), so A and B may be named differently.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.monoexp.com/bajdev/mpckit/internal/project"
	"git.monoexp.com/bajdev/mpckit/internal/xpj"
)

const maxShown = 40

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: xpjdiff A.xpj B.xpj")
		os.Exit(2)
	}
	n, err := compare(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "xpjdiff:", err)
		os.Exit(2)
	}
	if n > 0 {
		os.Exit(1)
	}
	fmt.Println("identical")
}

// compare prints the differences and returns how many there are.
func compare(a, b string) (int, error) {
	fa, err := xpj.ReadFile(a)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", a, err)
	}
	fb, err := xpj.ReadFile(b)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", b, err)
	}
	ea, err := fa.Encode()
	if err != nil {
		return 0, err
	}
	eb, err := fb.Encode()
	if err != nil {
		return 0, err
	}

	var diffs []string
	if !bytes.Equal(ea, eb) {
		if strings.Join(fa.Header, "\n") != strings.Join(fb.Header, "\n") {
			diffs = append(diffs, fmt.Sprintf("header: %q vs %q", fa.Header, fb.Header))
		}
		la, lb := map[string]string{}, map[string]string{}
		leaves(fa.Root, "", la)
		leaves(fb.Root, "", lb)
		for k, v := range la {
			if w, ok := lb[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("%s: only in A (%s)", k, v))
			} else if v != w {
				diffs = append(diffs, fmt.Sprintf("%s: %s vs %s", k, v, w))
			}
		}
		for k, v := range lb {
			if _, ok := la[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("%s: only in B (%s)", k, v))
			}
		}
		if len(diffs) == 0 {
			diffs = append(diffs, "same values, different key order or layout")
		}
	}
	diffs = append(diffs, compareData(a, b)...)

	sort.Strings(diffs)
	for i, d := range diffs {
		if i == maxShown {
			fmt.Printf("... and %d more\n", len(diffs)-maxShown)
			break
		}
		fmt.Println(d)
	}
	return len(diffs), nil
}

// leaves flattens a tree into path -> literal value.
func leaves(n *xpj.Node, path string, out map[string]string) {
	switch n.Kind {
	case xpj.Object:
		for _, m := range n.Members {
			leaves(m.Value, path+"."+m.Key, out)
		}
		if len(n.Members) == 0 {
			out[path] = "{}"
		}
	case xpj.Array:
		for i, it := range n.Items {
			leaves(it, fmt.Sprintf("%s[%d]", path, i), out)
		}
		if len(n.Items) == 0 {
			out[path] = "[]"
		}
	default:
		b, _ := n.MarshalJSON()
		out[path] = string(b)
	}
}

func dataDir(xpjPath string) string {
	return strings.TrimSuffix(xpjPath, filepath.Ext(xpjPath)) + project.DataSuffix
}

// compareData compares the files in the two data folders.
func compareData(a, b string) []string {
	da, db := dataDir(a), dataDir(b)
	ra, errA := os.ReadDir(da)
	rb, errB := os.ReadDir(db)
	if errA != nil || errB != nil {
		var out []string
		if errA != nil {
			out = append(out, "data folder A: "+errA.Error())
		}
		if errB != nil {
			out = append(out, "data folder B: "+errB.Error())
		}
		return out
	}
	names := map[string]int{}
	for _, e := range ra {
		names[e.Name()] |= 1
	}
	for _, e := range rb {
		names[e.Name()] |= 2
	}
	var out []string
	for name, in := range names {
		switch in {
		case 1:
			out = append(out, "data file only in A: "+name)
		case 2:
			out = append(out, "data file only in B: "+name)
		default:
			x, err1 := os.ReadFile(filepath.Join(da, name))
			y, err2 := os.ReadFile(filepath.Join(db, name))
			if err1 != nil || err2 != nil || !bytes.Equal(x, y) {
				out = append(out, "data file differs: "+name)
			}
		}
	}
	return out
}
