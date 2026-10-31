package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// maxFileSize is the largest user catalog file in bytes.
const maxFileSize = 1 << 20

// The errors of a Problem.
var (
	// ErrRead tells that a file or a directory could not be read.
	ErrRead = errors.New("i18n: cannot read the file")
	// ErrSyntax tells that a file is not a catalog: bad JSON, a value that
	// is not a text or a plural object, or a file that is too large.
	ErrSyntax = errors.New("i18n: the file is not a valid catalog")
	// ErrLanguage tells that a directory name is not a language code.
	ErrLanguage = errors.New("i18n: the directory name is not a language code")
)

// Problem is a user catalog file or directory that LoadDir did not use.
type Problem struct {
	// File is the path in the directory, for example "pt/core.json".
	File string
	// Err wraps ErrRead, ErrSyntax or ErrLanguage.
	Err error
}

// LoadDir loads the embedded catalogs and then the user catalogs in
// dir/<lang>/*.json. The files have the format of the embedded catalogs. A
// user text replaces the embedded text with the same ID. A directory of a
// new language adds that language. A text that a language does not have
// comes from Spanish. LoadDir does not use a bad file and adds a Problem for
// it. A missing dir gives the embedded catalogs only. The current language
// stays when it is still available.
func LoadDir(dir string) []Problem {
	b, _, err := load(locales)
	if err != nil {
		return []Problem{{File: "locales", Err: fmt.Errorf("%w: %v", ErrSyntax, err)}}
	}
	langs, problems := loadUser(b, dir)
	mu.Lock()
	bundle, languages = b, append(slices.Clone(builtin), langs...)
	lang := current
	mu.Unlock()
	SetLocale(lang)
	return problems
}

// loadUser adds the user catalogs to b. It returns the new languages in
// the order of their names, and the problems.
func loadUser(b *goi18n.Bundle, dir string) ([]string, []Problem) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []Problem{{File: dir, Err: fmt.Errorf("%w: %v", ErrRead, err)}}
	}
	var langs []string
	var problems []Problem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		lang := e.Name()
		if !validLanguage(lang) {
			problems = append(problems, Problem{File: lang, Err: ErrLanguage})
			continue
		}
		loaded, p := loadLanguage(b, filepath.Join(dir, lang), lang)
		problems = append(problems, p...)
		if loaded && !slices.Contains(builtin, lang) {
			langs = append(langs, lang)
		}
	}
	slices.Sort(langs)
	return langs, problems
}

// loadLanguage adds the *.json files of one language directory to b. It
// reports whether one file or more loaded.
func loadLanguage(b *goi18n.Bundle, dir, lang string) (bool, []Problem) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, []Problem{{File: lang, Err: fmt.Errorf("%w: %v", ErrRead, err)}}
	}
	loaded := false
	var problems []Problem
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".json" {
			continue
		}
		if err := loadFile(b, filepath.Join(dir, e.Name()), lang); err != nil {
			problems = append(problems, Problem{File: lang + "/" + e.Name(), Err: err})
			continue
		}
		loaded = true
	}
	return loaded, problems
}

func loadFile(b *goi18n.Bundle, name, lang string) error {
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRead, err)
	}
	defer func() { _ = f.Close() }()
	buf, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRead, err)
	}
	if len(buf) > maxFileSize {
		return fmt.Errorf("%w: larger than %d bytes", ErrSyntax, maxFileSize)
	}
	if _, err := b.ParseMessageBytes(language.Make(lang), buf, json.Unmarshal); err != nil {
		return fmt.Errorf("%w: %v", ErrSyntax, err)
	}
	return nil
}

// validLanguage reports whether name is a language code in its usual
// form, for example "pt" or "pt-BR".
func validLanguage(name string) bool {
	tag, err := language.Parse(name)
	return err == nil && tag.String() == name
}

// Export writes the embedded catalogs of lang to dir/<lang>/<area>.json,
// so that a translator has a complete file to start from. A language
// without embedded catalogs gets the Spanish catalogs. Export does not
// replace a file that is there: it returns that file in skipped. It returns
// the paths of the files that it wrote.
func Export(dir, lang string) (written, skipped []string, err error) {
	if !validLanguage(lang) {
		return nil, nil, fmt.Errorf("%w: %q", ErrLanguage, lang)
	}
	source := lang
	if !slices.Contains(builtin, lang) {
		source = defaultLocale
	}
	files, err := fs.ReadDir(locales, "locales/"+source)
	if err != nil {
		return nil, nil, err
	}
	out := filepath.Join(dir, lang)
	if err := os.MkdirAll(out, 0o700); err != nil {
		return nil, nil, err
	}
	for _, f := range files {
		target := filepath.Join(out, f.Name())
		done, err := exportFile("locales/"+source+"/"+f.Name(), target)
		switch {
		case err != nil:
			return written, skipped, err
		case done:
			written = append(written, target)
		default:
			skipped = append(skipped, target)
		}
	}
	return written, skipped, nil
}

// exportFile copies one embedded file to target. It returns false and no
// error when target is there.
func exportFile(source, target string) (bool, error) {
	buf, err := fs.ReadFile(locales, source)
	if err != nil {
		return false, err
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		return false, err
	}
	return true, f.Close()
}

// LanguageName returns the name of lang in that language, for example
// "português" for "pt". The catalog ID core.lang.<lang> wins when it is there.
func LanguageName(lang string) string {
	id := "core.lang." + lang
	if name := T(id); name != id {
		return name
	}
	if name := display.Self.Name(language.Make(lang)); name != "" {
		return name
	}
	return strings.ToUpper(lang)
}
