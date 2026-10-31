package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func renderBBS(t *testing.T, name string, width int) (string, []Link) {
	t.Helper()
	body, atts := loadFixture(t, name)
	out, links, err := New(baseURL).PostBBS(body, atts, width)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out, links
}

func TestPostBBSQuoteBox(t *testing.T) {
	out, _ := renderBBS(t, "quote-image", 60)
	lines := strings.Split(out, "\n")
	top := lineWith(lines, ".-- Soma Kurusu [1] escribió: ")
	if top < 0 {
		t.Fatalf("no quote box top:\n%s", out)
	}
	if !strings.HasSuffix(lines[top], "-.") || ansi.StringWidth(lines[top]) != 60 {
		t.Errorf("top line %q, want width 60 that ends in -.", lines[top])
	}
	if !strings.HasPrefix(lines[top+1], "| una señora recatada XD") || !strings.HasSuffix(lines[top+1], " |") {
		t.Errorf("quote line %q", lines[top+1])
	}
	if lines[top+2] != "'"+strings.Repeat("-", 58)+"'" {
		t.Errorf("bottom line %q", lines[top+2])
	}
	if lineWith(lines, ".-- Sherezada [3] escribió: ") < 0 {
		t.Errorf("no second quote box:\n%s", out)
	}
	if !strings.Contains(flat(out), "[imagen 2]") || !strings.Contains(flat(out), "Por eso nunca nos conocimos") {
		t.Errorf("text lost:\n%s", out)
	}
}

func TestPostBBSNestedQuotesAndCode(t *testing.T) {
	html := `<blockquote class="bb-quote"><cite class="bb-quote-author">ana</cite>` +
		`<blockquote class="bb-quote"><cite class="bb-quote-author">beto</cite>hola</blockquote>adios</blockquote>` +
		`<pre><code>x := 1</code></pre>`
	out, _, err := New(baseURL).PostBBS(html, nil, 30)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		".-- ana escribió: -----------.",
		"| .-- beto escribió: ------. |",
		"| | hola                   | |",
		"| '------------------------' |",
		"|                            |",
		"| adios                      |",
		"'----------------------------'",
		"",
		"+----------------------------+",
		"| x := 1                     |",
		"+----------------------------+",
	}, "\n")
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestPostBBSCodeBoxWithLanguage(t *testing.T) {
	out, _ := renderBBS(t, "code", 70)
	lines := strings.Split(out, "\n")
	top := lineWith(lines, "+-[ bash ]-")
	if top < 0 {
		t.Fatalf("no code box with language:\n%s", out)
	}
	if !strings.HasSuffix(lines[top], "-+") || ansi.StringWidth(lines[top]) != 70 {
		t.Errorf("top line %q", lines[top])
	}
	if !strings.HasPrefix(lines[top+1], "| sudo apt install wine") || !strings.HasSuffix(lines[top+1], " |") {
		t.Errorf("code line %q", lines[top+1])
	}
	if lines[top+2] != "+"+strings.Repeat("-", 68)+"+" {
		t.Errorf("bottom line %q", lines[top+2])
	}
	if lineWith(lines, "| ./setup_dxvk.sh install") < 0 {
		t.Errorf("multi-line code lost:\n%s", out)
	}
}

func TestPostBBSNoEscapeAndWidth(t *testing.T) {
	for _, name := range fixtureNames(t) {
		for _, width := range []int{40, 80} {
			out, _ := renderBBS(t, name, width)
			if strings.Contains(out, "\x1b") {
				t.Errorf("%s: output has ESC", name)
			}
			for _, line := range strings.Split(out, "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("%s width %d: line has width %d: %q", name, width, w, line)
				}
			}
		}
	}
}

func TestPostBBSLinksMatchPost(t *testing.T) {
	for _, name := range fixtureNames(t) {
		_, want := render(t, name, 80, "notty")
		_, got := renderBBS(t, name, 80)
		assertLinks(t, got, want)
	}
}

func lineWith(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}
