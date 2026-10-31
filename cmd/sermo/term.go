package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/term"
)

// defaultWidth is the line width when the output is not a terminal.
const defaultWidth = 80

// isTerminal reports whether w is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

// termWidth returns the width of the terminal w, or defaultWidth.
func termWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return defaultWidth
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}

// startWait shows text with a spinner on a terminal, or as one line on
// other outputs. The returned function stops the spinner and clears the line.
func startWait(w io.Writer, text string) func() {
	if !isTerminal(w) {
		_, _ = fmt.Fprintln(w, text)
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		spin(w, text, done)
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

func spin(w io.Writer, text string, done <-chan struct{}) {
	frames := []string{"|", "/", "-", "\\"}
	t := time.NewTicker(120 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		_, _ = fmt.Fprintf(w, "\r%s %s", frames[i%len(frames)], text)
		select {
		case <-done:
			_, _ = fmt.Fprintf(w, "\r%s\r", strings.Repeat(" ", utf8.RuneCountInString(text)+2))
			return
		case <-t.C:
		}
	}
}
