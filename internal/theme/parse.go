package theme

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/fx"
)

// The errors of a theme file. Load wraps them in Problem.Err.
var (
	ErrRead     = errors.New("theme: the file cannot be read")
	ErrTooLarge = errors.New("theme: the file is too large")
	ErrSyntax   = errors.New("theme: the file is not valid JSON for a theme")
	ErrID       = errors.New("theme: the file name is not a valid theme ID")
	ErrMissing  = errors.New("theme: a field is missing")
	ErrColor    = errors.New("theme: a color is not #rrggbb")
	ErrGradient = errors.New("theme: a gradient has fewer than 2 colors")
	ErrBorder   = errors.New("theme: the border style is unknown")
	ErrFont     = errors.New("theme: the banner font is unknown")
	ErrEffect   = errors.New("theme: the effect is unknown")
	ErrContrast = errors.New("theme: a color is hard to read on a dark background")
)

// borderStyles are the values of the border field. The first value is the
// default.
var borderStyles = []string{"double", "normal", "rounded", "thick", "ascii"}

var (
	idPattern  = regexp.MustCompile(`^[a-z0-9-]+$`)
	hexPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// file is the JSON format of a theme file.
type file struct {
	Name   string `json:"name"`
	Colors struct {
		Text      string `json:"text"`
		Muted     string `json:"muted"`
		Accent    string `json:"accent"`
		Error     string `json:"error"`
		Success   string `json:"success"`
		Warning   string `json:"warning"`
		Selection string `json:"selection"`
	} `json:"colors"`
	Gradients struct {
		Border    []string `json:"border"`
		Title     []string `json:"title"`
		Highlight []string `json:"highlight"`
		Splash    []string `json:"splash"`
	} `json:"gradients"`
	Border     string `json:"border"`
	BannerFont string `json:"bannerFont"`
	Effect     string `json:"effect"`
}

// parse decodes and checks the theme file data. The id is the file name
// without ".json".
func parse(id string, data []byte) (Theme, error) {
	if !idPattern.MatchString(id) {
		return Theme{}, fmt.Errorf("%w: %q", ErrID, id)
	}
	var f file
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Theme{}, fmt.Errorf("%w: %v", ErrSyntax, err)
	}
	t, err := f.theme(id)
	if err != nil {
		return Theme{}, err
	}
	if err := checkOptions(t); err != nil {
		return Theme{}, err
	}
	if err := readable(t); err != nil {
		return Theme{}, err
	}
	return t, nil
}

func (f file) theme(id string) (Theme, error) {
	if strings.TrimSpace(f.Name) == "" {
		return Theme{}, fmt.Errorf("%w: name", ErrMissing)
	}
	t := Theme{ID: id, Name: f.Name, BorderStyle: f.Border, BannerFont: f.BannerFont, Effect: f.Effect}
	if t.BorderStyle == "" {
		t.BorderStyle = borderStyles[0]
	}
	var p parser
	c := f.Colors
	t.Text = p.color("colors.text", c.Text)
	t.Muted = p.color("colors.muted", c.Muted)
	t.Accent = p.color("colors.accent", c.Accent)
	t.Error = p.color("colors.error", c.Error)
	t.Success = p.color("colors.success", c.Success)
	t.Warning = p.color("colors.warning", c.Warning)
	t.Selection = p.color("colors.selection", c.Selection)
	g := f.Gradients
	t.Border = p.gradient("gradients.border", g.Border)
	t.Title = p.gradient("gradients.title", g.Title)
	t.Highlight = p.gradient("gradients.highlight", g.Highlight)
	t.Splash = p.gradient("gradients.splash", g.Splash)
	return t, p.err
}

// parser keeps the first error of a sequence of color parses.
type parser struct{ err error }

func (p *parser) color(field, s string) colorful.Color {
	if p.err != nil {
		return colorful.Color{}
	}
	if s == "" {
		p.err = fmt.Errorf("%w: %s", ErrMissing, field)
		return colorful.Color{}
	}
	if !hexPattern.MatchString(s) {
		p.err = fmt.Errorf("%w: %s is %q", ErrColor, field, s)
		return colorful.Color{}
	}
	c, err := colorful.Hex(s)
	if err != nil {
		p.err = fmt.Errorf("%w: %s is %q", ErrColor, field, s)
	}
	return c
}

func (p *parser) gradient(field string, stops []string) fx.Gradient {
	if p.err != nil {
		return nil
	}
	switch len(stops) {
	case 0:
		p.err = fmt.Errorf("%w: %s", ErrMissing, field)
		return nil
	case 1:
		p.err = fmt.Errorf("%w: %s", ErrGradient, field)
		return nil
	}
	g := make(fx.Gradient, len(stops))
	for i, s := range stops {
		g[i] = p.color(fmt.Sprintf("%s[%d]", field, i), s)
	}
	return g
}

// checkOptions checks the optional fields.
func checkOptions(t Theme) error {
	if !slices.Contains(borderStyles, t.BorderStyle) {
		return fmt.Errorf("%w: %q", ErrBorder, t.BorderStyle)
	}
	if t.BannerFont != "" && !banner.HasFont(t.BannerFont) {
		return fmt.Errorf("%w: %q", ErrFont, t.BannerFont)
	}
	if t.Effect != "" {
		if _, ok := fx.ByName(t.Effect, t.Splash); !ok {
			return fmt.Errorf("%w: %q", ErrEffect, t.Effect)
		}
	}
	return nil
}
