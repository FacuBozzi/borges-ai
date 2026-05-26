package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// dirEntry is one row in a dirList: a subdirectory, a matching file, or the
// synthetic ".." parent row.
type dirEntry struct {
	name  string
	path  string // absolute
	isDir bool
	isUp  bool
}

// dirList is a reusable directory browser built on widget.List. It shows the
// subdirectories of the current directory plus files whose extension is in
// exts, dirs-first, with a ".." row except at the filesystem root. Selecting a
// directory (or "..") descends/ascends; selecting a file stages it and invokes
// onFile. It is not a widget itself — embed dl.list in a container.
type dirList struct {
	exts    []string
	dir     string
	all     []dirEntry // unfiltered
	entries []dirEntry // visible after filter
	filter  string
	selFile string // staged file path ("" when none / selection is a dir)
	list    *widget.List

	onFile func(path string) // file row selected (staged, not confirmed)
	onDir  func(dir string)  // current directory changed
	onErr  func(err error)   // os.ReadDir failed
}

func newDirList(exts []string) *dirList {
	dl := &dirList{exts: exts}
	dl.list = widget.NewList(
		func() int { return len(dl.entries) },
		func() fyne.CanvasObject { return widget.NewLabel("entry") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			e := dl.entries[i]
			switch {
			case e.isUp:
				lbl.SetText("../")
			case e.isDir:
				lbl.SetText(e.name + "/")
			default:
				lbl.SetText(e.name)
			}
		},
	)
	dl.list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(dl.entries) {
			return
		}
		e := dl.entries[i]
		if e.isDir || e.isUp {
			dl.navigate(e.path)
			return
		}
		dl.selFile = e.path
		if dl.onFile != nil {
			dl.onFile(e.path)
		}
	}
	return dl
}

// navigate reads dir and makes it current. On read error it stays put and
// reports via onErr.
func (dl *dirList) navigate(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if dl.onErr != nil {
			dl.onErr(err)
		}
		return
	}
	dl.dir = dir
	dl.all = dl.buildEntries(dir, ents)
	dl.selFile = ""
	dl.applyFilter()
	dl.list.UnselectAll()
	dl.list.ScrollToTop()
	if dl.onDir != nil {
		dl.onDir(dir)
	}
}

func (dl *dirList) buildEntries(dir string, ents []os.DirEntry) []dirEntry {
	var dirs, files []dirEntry
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") { // skip hidden
			continue
		}
		full := filepath.Join(dir, name)
		switch {
		case e.IsDir():
			dirs = append(dirs, dirEntry{name: name, path: full, isDir: true})
		case dl.matchExt(name):
			files = append(files, dirEntry{name: name, path: full})
		}
	}
	byName := func(s []dirEntry) {
		sort.Slice(s, func(i, j int) bool {
			return strings.ToLower(s[i].name) < strings.ToLower(s[j].name)
		})
	}
	byName(dirs)
	byName(files)

	out := make([]dirEntry, 0, len(dirs)+len(files)+1)
	if parent := filepath.Dir(dir); parent != dir {
		out = append(out, dirEntry{name: "..", path: parent, isUp: true})
	}
	out = append(out, dirs...)
	out = append(out, files...)
	return out
}

func (dl *dirList) matchExt(name string) bool {
	if len(dl.exts) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range dl.exts {
		if ext == strings.ToLower(e) {
			return true
		}
	}
	return false
}

func (dl *dirList) setFilter(q string) {
	dl.filter = q
	dl.applyFilter()
}

func (dl *dirList) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(dl.filter))
	if q == "" {
		dl.entries = dl.all
	} else {
		out := make([]dirEntry, 0, len(dl.all))
		for _, e := range dl.all {
			if e.isUp || strings.Contains(strings.ToLower(e.name), q) {
				out = append(out, e)
			}
		}
		dl.entries = out
	}
	dl.list.Refresh()
}

// chosenFile returns the staged file, or the first visible file when nothing
// is explicitly selected (so Enter on a freshly-opened dir picks sensibly).
func (dl *dirList) chosenFile() string {
	if dl.selFile != "" {
		return dl.selFile
	}
	for _, e := range dl.entries {
		if !e.isDir && !e.isUp {
			return e.path
		}
	}
	return ""
}
