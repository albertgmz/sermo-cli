package bbcode

import "testing"

func TestQuote(t *testing.T) {
	cases := []struct {
		name, author string
		post, member int
		body, want   string
	}{
		{"post", "ember", 5, 9, "Hello\n\nWorld", `[QUOTE="ember, post: 5, member: 9"]Hello` + "\n\n" + `World[/QUOTE]`},
		{"trims body", "ember", 5, 9, "\n  Hi \n", `[QUOTE="ember, post: 5, member: 9"]Hi[/QUOTE]`},
		{"guest has no member", "Wanderer", 5, 0, "Hi", `[QUOTE="Wanderer, post: 5"]Hi[/QUOTE]`},
		{"name is cleaned", "a\"b], c\nd", 1, 2, "x", `[QUOTE="a b c d, post: 1, member: 2"]x[/QUOTE]`},
		{"empty name is left out", `"]`, 1, 2, "x", `[QUOTE="post: 1, member: 2"]x[/QUOTE]`},
		{"name with spaces", "Soma Kurusu", 467171, 45, "una señora", `[QUOTE="Soma Kurusu, post: 467171, member: 45"]una señora[/QUOTE]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Quote(c.author, c.post, c.member, c.body); got != c.want {
				t.Fatalf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestQuoteKeepsBalancedNestedQuotes(t *testing.T) {
	body := `[QUOTE="ana, post: 1"]old[/QUOTE]new`
	want := `[QUOTE="ember, post: 5, member: 9"][QUOTE="ana, post: 1"]old[/QUOTE]new[/QUOTE]`
	if got := Quote("ember", 5, 9, body); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestQuoteMakesUnpairedQuoteTagsPlain(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"lone closer", "one [/QUOTE] two", "one [PLAIN][/QUOTE][/PLAIN] two"},
		{"unclosed opener", "one [quote=x] two", "one [PLAIN][quote=x][/PLAIN] two"},
		{"closer in code stays", "[CODE][/QUOTE][/CODE]", "[CODE][/QUOTE][/CODE]"},
		{"unclosed code", "[CODE]a [/QUOTE]", "[PLAIN][CODE][/PLAIN]a [PLAIN][/QUOTE][/PLAIN]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := `[QUOTE="ember, post: 5, member: 9"]` + c.want + `[/QUOTE]`
			if got := Quote("ember", 5, 9, c.body); got != want {
				t.Fatalf("got  %q\nwant %q", got, want)
			}
		})
	}
}

func TestWrapSelection(t *testing.T) {
	cases := []struct {
		tag        string
		start, end int
		want       string
		cursor     int
	}{
		{"B", 5, 10, "hola [B]mundo[/B]!", 17},
		{"i", 5, 10, "hola [I]mundo[/I]!", 17},
		{"U", 5, 10, "hola [U]mundo[/U]!", 17},
		{"S", 5, 10, "hola [S]mundo[/S]!", 17},
		{"URL", 5, 10, "hola [URL]mundo[/URL]!", 21},
		{"CODE", 5, 10, "hola [CODE]mundo[/CODE]!", 23},
		{"SPOILER", 5, 10, "hola [SPOILER]mundo[/SPOILER]!", 29},
		{"QUOTE", 5, 10, "hola [QUOTE]mundo[/QUOTE]!", 25},
	}
	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			got, cur := Wrap("hola mundo!", c.tag, c.start, c.end)
			if got != c.want || cur != c.cursor {
				t.Fatalf("got %q %d, want %q %d", got, cur, c.want, c.cursor)
			}
		})
	}
}

func TestWrapEmptySelectionPutsCursorInside(t *testing.T) {
	got, cur := Wrap("ab", "B", 1, 1)
	if got != "a[B][/B]b" || cur != 4 {
		t.Fatalf("got %q %d", got, cur)
	}
}

func TestWrapCountsRunes(t *testing.T) {
	got, cur := Wrap("año ñu", "B", 4, 6)
	if got != "año [B]ñu[/B]" || cur != 13 {
		t.Fatalf("got %q %d", got, cur)
	}
}

func TestWrapFixesBadOffsets(t *testing.T) {
	got, cur := Wrap("abc", "B", 5, -1)
	if got != "[B]abc[/B]" || cur != 10 {
		t.Fatalf("got %q %d", got, cur)
	}
}
