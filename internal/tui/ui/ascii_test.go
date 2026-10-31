package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

func TestTableGolden(t *testing.T) {
	tb := NewTable(40, Column{Title: "##", Width: 2, Align: AlignRight}, Column{Title: "Foro"},
		Column{Title: "Hilos", Width: 5, Align: AlignRight})
	tb.Full("INDICE").Header().Section("GENERAL").
		Row("01", "(*) Anuncios", "24").Sub("", "Noticias oficiales del sitio largo largo", "").
		Row("02", "Presentaciones", "2,907").Rule().
		Section("COMUNIDAD").Row("03", "Off-topic", "9").
		Full("(*) nuevos").Full("En linea (3): a, b")
	want := strings.Join([]string{
		"+--------------------------------------+",
		"| INDICE                               |",
		"+----+-------------------------+-------+",
		"| ## | Foro                    | Hilos |",
		"+====+=========================+=======+",
		"|    |  ===  GENERAL  ===              |",
		"| 01 | (*) Anuncios            |    24 |",
		"|    | Noticias oficiales d... |       |",
		"| 02 | Presentaciones          | 2,907 |",
		"+----+-------------------------+-------+",
		"|    |  ===  COMUNIDAD  ===            |",
		"| 03 | Off-topic               |     9 |",
		"+----+-------------------------+-------+",
		"| (*) nuevos                           |",
		"| En linea (3): a, b                   |",
		"+--------------------------------------+",
	}, "\n")
	if got := tb.Render(Styles{}); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTableWidthWithAccentsAndColor(t *testing.T) {
	s := NewStyles(theme.ByID(""))
	tb := NewTable(30, Column{Title: "Tema"}, Column{Title: "Por", Width: 8, Align: AlignCenter})
	tb.Header().Row(s.Accent.Render("Programación y más cosas aquí"), "ñandú").Select().Full("señal 😏")
	out := tb.Render(s)
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w != 30 {
			t.Errorf("line width %d, want 30: %q", w, ansi.Strip(line))
		}
	}
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "| Programación... |  ñandú   |") {
		t.Errorf("cells not cut or centered:\n%s", plain)
	}
}

func TestBoxes(t *testing.T) {
	if got, want := Box(Styles{}, 12, []string{"hola", "una línea larga"}), "+----------+\n| hola     |\n| una l... |\n+----------+"; got != want {
		t.Errorf("Box:\n%s\nwant:\n%s", got, want)
	}
	if got, want := DoubleBox(Styles{}, 8, []string{"", "ab"}), "+======+\n|      |\n| ab   |\n+======+"; got != want {
		t.Errorf("DoubleBox:\n%s\nwant:\n%s", got, want)
	}
	if got := Rule(Styles{}, 6, '='); got != "+====+" {
		t.Errorf("Rule = %q", got)
	}
}

func TestNum(t *testing.T) {
	for _, c := range []struct {
		i, total int
		want     string
	}{{1, 7, "01"}, {12, 99, "12"}, {7, 120, "007"}, {1208, 1208, "1208"}} {
		if got := Num(c.i, c.total); got != c.want {
			t.Errorf("Num(%d, %d) = %q, want %q", c.i, c.total, got, c.want)
		}
	}
}

func TestDepsBBS(t *testing.T) {
	d := &Deps{}
	if d.BBS() {
		t.Error("no config must give the modern interface")
	}
	d.Config = &config.Config{Interface: config.InterfaceBBS}
	if !d.BBS() {
		t.Error("interface bbs must give the BBS interface")
	}
}
