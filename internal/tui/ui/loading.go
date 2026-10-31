package ui

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// NewSpinner returns a spinner in the accent color. Start its ticks with
// spinner.Model.Tick only when Deps.Animate is true. Without ticks it shows
// one static frame.
func NewSpinner(s Styles) spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(s.Accent))
}

// Loading returns the spinner and text in the center of a width x height
// area. An empty text gives the default text.
func Loading(s Styles, sp spinner.Model, text string, width, height int) string {
	if text == "" {
		text = i18n.T("core.loading")
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, sp.View()+" "+s.Muted.Render(text))
}

// Empty returns text in the center of a width x height area, for a list
// with no items.
func Empty(s Styles, text string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, s.Muted.Render(text))
}
