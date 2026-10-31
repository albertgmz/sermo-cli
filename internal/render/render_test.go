package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

const baseURL = "https://fcuba.net"

type fixture struct {
	BodyHTML    string `json:"bodyHtml"`
	Attachments []struct {
		FileID      int    `json:"fileId"`
		URL         string `json:"url"`
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
	} `json:"attachments"`
}

func loadFixture(t *testing.T, name string) (string, []Attachment) {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(buf, &f); err != nil {
		t.Fatal(err)
	}
	atts := make([]Attachment, 0, len(f.Attachments))
	for _, a := range f.Attachments {
		atts = append(atts, Attachment{ID: a.FileID, URL: a.URL, Filename: a.Filename, ContentType: a.ContentType})
	}
	return f.BodyHTML, atts
}

func fixtureNames(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, strings.TrimSuffix(filepath.Base(p), ".json"))
	}
	return names
}

// flat returns the visible text with all white space runs made one space.
func flat(s string) string {
	return strings.Join(strings.Fields(ansi.Strip(s)), " ")
}

func render(t *testing.T, name string, width int, style string) (string, []Link) {
	t.Helper()
	body, atts := loadFixture(t, name)
	out, links, err := New(baseURL).Post(body, atts, width, style)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out, links
}

func TestFixturesRender(t *testing.T) {
	for _, name := range fixtureNames(t) {
		for _, style := range []string{"dark", "light", "notty"} {
			out, _ := render(t, name, 80, style)
			if strings.TrimSpace(ansi.Strip(out)) == "" {
				t.Errorf("%s %s: empty output", name, style)
			}
		}
	}
}

func TestImageLabelLinkNumbersAndQuotes(t *testing.T) {
	out, links := render(t, "quote-image", 100, "notty")
	text := flat(out)
	for _, want := range []string{
		"Soma Kurusu [1] escribió:",
		"una señora recatada XD",
		"[imagen 2]",
		"Sherezada [3] escribió:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	want := []Link{
		{N: 1, Label: "Soma Kurusu", URL: "https://fcuba.net/posts/467171", Kind: LinkURL},
		{N: 2, Label: "", URL: "https://foroscuba.net/attachments/screenshot_20250119_093403_facebook-webp.19612/", Kind: LinkImage},
		{N: 3, Label: "Sherezada", URL: "https://fcuba.net/posts/467176", Kind: LinkURL},
	}
	assertLinks(t, links, want)
}

func TestImageWithAlt(t *testing.T) {
	out, links, err := New(baseURL).Post(`<p>a <img src="/i.png" alt="gato"> b</p>`, nil, 80, "notty")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flat(out), "a [imagen 1: gato] b") {
		t.Errorf("output: %q", flat(out))
	}
	assertLinks(t, links, []Link{{N: 1, Label: "gato", URL: "https://fcuba.net/i.png", Kind: LinkImage}})
}

func TestLinkURLsAndOrder(t *testing.T) {
	html := `<p><a href="/members/7">@Kira</a> y <a href="https://example.com/b?x=1">sitio</a>.</p>` +
		`<p><a href="javascript:alert(1)">malo</a> <a href="mailto:ana@example.com">correo</a> <a href="posts/9">rel</a></p>`
	out, links, err := New("https://fcuba.net/threads/").Post(html, nil, 80, "notty")
	if err != nil {
		t.Fatal(err)
	}
	text := flat(out)
	for _, want := range []string{"@Kira [1] y sitio [2].", "malo correo [3] rel [4]"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	assertLinks(t, links, []Link{
		{N: 1, Label: "@Kira", URL: "https://fcuba.net/members/7", Kind: LinkURL},
		{N: 2, Label: "sitio", URL: "https://example.com/b?x=1", Kind: LinkURL},
		{N: 3, Label: "correo", URL: "mailto:ana@example.com", Kind: LinkURL},
		{N: 4, Label: "rel", URL: "https://fcuba.net/threads/posts/9", Kind: LinkURL},
	})
}

func TestInlineAndListedAttachments(t *testing.T) {
	out, links := render(t, "spoiler-list", 100, "notty")
	text := flat(out)
	inline := strings.Index(text, "[adjunto 2: 1692205418852.webp]")
	header := strings.Index(text, "2 adjuntos")
	listed := strings.Index(text, "[adjunto 3: 1692206078588.webp]")
	last := strings.Index(text, "[adjunto 4: Screenshot_20230816_131357.webp]")
	if inline < 0 || header < inline || listed < header || last < listed {
		t.Fatalf("attachments out of place (%d %d %d %d):\n%s", inline, header, listed, last, text)
	}
	if strings.Contains(text, "11991") {
		t.Errorf("raw attachment id in output:\n%s", text)
	}
	assertLinks(t, links, []Link{
		{N: 1, Label: "https://foroscuba.net/cuenta/preferences", URL: "https://foroscuba.net/cuenta/preferences", Kind: LinkURL},
		{N: 2, Label: "1692205418852.webp", URL: "https://fcuba.net/api/v1/files/11991", Kind: LinkAttachment},
		{N: 3, Label: "1692206078588.webp", URL: "https://fcuba.net/api/v1/files/11998", Kind: LinkAttachment},
		{N: 4, Label: "Screenshot_20230816_131357.webp", URL: "https://fcuba.net/api/v1/files/11999", Kind: LinkAttachment},
	})
}

func TestAttachmentsWithoutBody(t *testing.T) {
	atts := []Attachment{{ID: 1, URL: "/api/v1/files/1", Filename: "a.pdf", ContentType: "application/pdf"}}
	out, links, err := New(baseURL).Post("", atts, 80, "notty")
	if err != nil {
		t.Fatal(err)
	}
	if text := flat(out); !strings.Contains(text, "1 adjunto") || !strings.Contains(text, "[adjunto 1: a.pdf]") {
		t.Errorf("output: %q", text)
	}
	assertLinks(t, links, []Link{{N: 1, Label: "a.pdf", URL: "https://fcuba.net/api/v1/files/1", Kind: LinkAttachment}})
}

func TestSpoilerLabels(t *testing.T) {
	out, _ := render(t, "spoilers", 100, "notty")
	text := flat(out)
	for _, want := range []string{"[spoiler: Agroplt 2015 aprox]", "[spoiler: Alterego el 7 May 2021]"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
	out, _, err := New(baseURL).Post(`<details class="spoiler"><summary class="bb-spoiler-summary"></summary><p>secreto</p></details><p>x <span class="spoiler">oculto</span></p>`, nil, 80, "notty")
	if err != nil {
		t.Fatal(err)
	}
	if text := flat(out); !strings.Contains(text, "[spoiler] secreto") || !strings.Contains(text, "x [spoiler] oculto") {
		t.Errorf("output: %q", text)
	}
}

func TestCodeBlockKeepsCode(t *testing.T) {
	out, _ := render(t, "code", 100, "notty")
	if !strings.Contains(ansi.Strip(out), "sudo apt install wine") {
		t.Errorf("code lost:\n%s", out)
	}
}

func TestNoTTYHasNoEscape(t *testing.T) {
	for _, name := range fixtureNames(t) {
		out, _ := render(t, name, 80, "notty")
		if strings.Contains(out, "\x1b") {
			t.Errorf("%s: notty output has ESC", name)
		}
	}
}

func TestWidthRespected(t *testing.T) {
	for _, name := range fixtureNames(t) {
		for _, width := range []int{50, 80} {
			for _, style := range []string{"dark", "notty"} {
				out, _ := render(t, name, width, style)
				for _, line := range strings.Split(out, "\n") {
					if w := ansi.StringWidth(line); w > width {
						t.Errorf("%s %s width %d: line has width %d: %q", name, style, width, w, ansi.Strip(line))
					}
				}
			}
		}
	}
}

func TestNoSurroundingBlankLines(t *testing.T) {
	for _, style := range []string{"dark", "notty"} {
		out, _ := render(t, "quote-image", 80, style)
		lines := strings.Split(out, "\n")
		if strings.TrimSpace(ansi.Strip(lines[0])) == "" || strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
			t.Errorf("%s: blank first or last line: %q", style, out)
		}
	}
}

func TestEnglishLabels(t *testing.T) {
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale("es") })
	out, _ := render(t, "quote-image", 100, "notty")
	if text := flat(out); !strings.Contains(text, "Soma Kurusu [1] wrote:") || !strings.Contains(text, "[image 2]") {
		t.Errorf("output: %s", text)
	}
}

func TestUnknownStyleFails(t *testing.T) {
	if _, _, err := New(baseURL).Post("<p>x</p>", nil, 80, "neon"); err == nil {
		t.Fatal("want error for unknown style")
	}
}

func assertLinks(t *testing.T, got, want []Link) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d links %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
