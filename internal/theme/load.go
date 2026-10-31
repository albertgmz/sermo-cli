package theme

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxFileSize is the largest theme file in bytes.
const maxFileSize = 64 << 10

// Problem is a theme file that Load did not use.
type Problem struct {
	// File is the file name in the directory.
	File string
	// Err wraps one of the Err values of this package.
	Err error
}

// Load reads each *.json file in dir as a user theme. The ID of a theme is
// the file name without ".json". The set has the built-in themes first and
// then the user themes in the order of their names. A user theme with the
// ID of a built-in theme replaces it. Load does not use a bad file and adds
// a Problem for it. A missing dir gives the built-in themes and no problems.
func Load(dir string) (Set, []Problem) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return Builtin(), nil
	}
	if err != nil {
		return Builtin(), []Problem{{File: dir, Err: fmt.Errorf("%w: %v", ErrRead, err)}}
	}
	var user []Theme
	var problems []Problem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		t, err := loadFile(dir, e.Name())
		if err != nil {
			problems = append(problems, Problem{File: e.Name(), Err: err})
			continue
		}
		user = append(user, t)
	}
	return merge(user), problems
}

func loadFile(dir, name string) (Theme, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return Theme{}, fmt.Errorf("%w: %v", ErrRead, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return Theme{}, fmt.Errorf("%w: %v", ErrRead, err)
	}
	if len(data) > maxFileSize {
		return Theme{}, fmt.Errorf("%w: the maximum is %d bytes", ErrTooLarge, maxFileSize)
	}
	t, err := parse(strings.TrimSuffix(name, ".json"), data)
	t.Custom = true
	return t, err
}

// merge puts the user themes into the built-in themes.
func merge(user []Theme) Set {
	out := slices.Clone(builtins)
	var added []Theme
	for _, t := range user {
		if i := slices.IndexFunc(out, func(b Theme) bool { return b.ID == t.ID }); i >= 0 {
			out[i] = t
			continue
		}
		added = append(added, t)
	}
	slices.SortStableFunc(added, func(a, b Theme) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return Set{themes: append(out, added...)}
}
