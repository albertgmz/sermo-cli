package ui

import tea "charm.land/bubbletea/v2"

// Command is one command of the BBS prompt.
type Command struct {
	// Name is the word that the user types, for example "R". The app
	// matches it without case. "#" matches a number, with the digits as
	// the argument. "" matches a bare Enter.
	Name string
	// Label is the hint in the hint line, for example "[R]esponder". The
	// text in brackets gets the Accent style. An empty Label hides the
	// command from the hint line.
	Label string
	// Run runs the command. Arg is the text after the name, or the digits
	// of a number. It is "" when the user typed no argument.
	Run func(arg string) tea.Cmd
}

// Commander is an optional interface of a Screen. In the BBS interface the
// app shows the commands in the hint line and runs them from the prompt.
// The commands of the screen come before the global commands of the app.
type Commander interface {
	// Commands returns the commands of the screen in the order of the
	// hint line.
	Commands() []Command
	// Prompt returns the label of the prompt, for example "Leyendo".
	Prompt() string
}
