// Package i18n gives the localized text for the user interface.
//
// The catalogs are JSON files in locales/<lang>/<area>.json, for example
// locales/es/forum.json and locales/en/forum.json. The name of the
// directory sets the language. Every *.json file in locales/es and
// locales/en loads. Each area owns one file with the same name in each
// language. A file holds a flat object of dotted IDs. A plain value is a
// text. An object with "one" and "other" keys is a plural text. Texts use
// Go template data, for example {{.Name}}. Every ID must be in all the
// languages.
package i18n

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"sync"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const defaultLocale = "es"

var supported = map[string]bool{"es": true, "en": true}

//go:embed locales
var locales embed.FS

var (
	bundle    *goi18n.Bundle
	mu        sync.RWMutex
	current   string
	localizer *goi18n.Localizer
)

func init() {
	b, _, err := load(locales)
	if err != nil {
		panic(err)
	}
	bundle = b
	SetLocale(defaultLocale)
}

// load reads every catalog file in fsys. It returns the bundle and the
// message IDs of each language.
func load(fsys fs.FS) (*goi18n.Bundle, map[string]map[string]bool, error) {
	b := goi18n.NewBundle(language.Spanish)
	ids := map[string]map[string]bool{}
	err := fs.WalkDir(fsys, "locales", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".json" {
			return err
		}
		lang := strings.Split(p, "/")[1]
		buf, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		msgs, err := b.ParseMessageBytes(language.Make(lang), buf, json.Unmarshal)
		if err != nil {
			return &fs.PathError{Op: "parse", Path: p, Err: err}
		}
		if ids[lang] == nil {
			ids[lang] = map[string]bool{}
		}
		for _, m := range msgs {
			ids[lang][m.ID] = true
		}
		return nil
	})
	return b, ids, err
}

// catalogIDs returns the message IDs of each language in the embedded catalogs.
func catalogIDs() (map[string]map[string]bool, error) {
	_, ids, err := load(locales)
	return ids, err
}

// SetLocale sets the language of all texts. The values are "es" and "en".
// An unknown value sets "es".
func SetLocale(lang string) {
	if !supported[lang] {
		lang = defaultLocale
	}
	mu.Lock()
	defer mu.Unlock()
	current = lang
	localizer = goi18n.NewLocalizer(bundle, lang)
}

// Locale returns the current language.
func Locale() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T returns the text for id. It returns id when the catalogs do not have it.
func T(id string) string {
	return localize(&goi18n.LocalizeConfig{MessageID: id})
}

// Tf returns the text for id with the template data.
func Tf(id string, data map[string]any) string {
	return localize(&goi18n.LocalizeConfig{MessageID: id, TemplateData: data})
}

// Plural returns the plural form of id for count. The template data
// gets the key Count with the value of count.
func Plural(id string, count int, data map[string]any) string {
	all := make(map[string]any, len(data)+1)
	for k, v := range data {
		all[k] = v
	}
	all["Count"] = count
	return localize(&goi18n.LocalizeConfig{MessageID: id, TemplateData: all, PluralCount: count})
}

func localize(lc *goi18n.LocalizeConfig) string {
	mu.RLock()
	l := localizer
	mu.RUnlock()
	msg, err := l.Localize(lc)
	if err != nil && msg == "" {
		return lc.MessageID
	}
	return msg
}
