package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/auth"
)

func TestBannedOnMeDeletesDeviceTokenAndOffersLogin(t *testing.T) {
	f := newFakeSermo(t)
	f.meStatus.Store(403)
	d := testDeps(t, f, testToken)
	if err := d.Store.Save(f.URL, auth.Token{AccessToken: testToken}); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, err := d.Store.Load(f.URL); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("token still stored: %v", err)
	}
	if d.Cred.Token != "" || d.Me != nil {
		t.Fatalf("still signed in: %+v", d.Cred)
	}
	out := screenText(m)
	for _, want := range []string{"Tu cuenta está suspendida. La sesión de este dispositivo terminó.", "[Enter] Entrar", "Invitado"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen misses %q:\n%s", want, out)
		}
	}
}

func TestBannedOnCountersSignsOut(t *testing.T) {
	f := newFakeSermo(t)
	f.counterStatus.Store(403)
	d := testDeps(t, f, testToken)
	if err := d.Store.Save(f.URL, auth.Token{AccessToken: testToken}); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, err := d.Store.Load(f.URL); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("token still stored: %v", err)
	}
	if d.Cred.Token != "" {
		t.Fatalf("still signed in: %+v", d.Cred)
	}
	if out := screenText(m); !strings.Contains(out, "Tu cuenta está suspendida.") {
		t.Errorf("no banned reason:\n%s", out)
	}
}

func TestBannedWithAPIKeyKeepsStoredTokenAndSaysNothingAboutTheDevice(t *testing.T) {
	f := newFakeSermo(t)
	f.meStatus.Store(403)
	d := testDeps(t, f, testToken)
	d.Cred.Source = auth.SourceAPIKey
	if err := d.Store.Save(f.URL, auth.Token{AccessToken: "other"}); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, err := d.Store.Load(f.URL); err != nil {
		t.Fatalf("a stored device token must stay for an API key: %v", err)
	}
	if d.Cred.Token != "" {
		t.Fatalf("still signed in: %+v", d.Cred)
	}
	out := screenText(m)
	if !strings.Contains(out, "Tu cuenta está suspendida, así que no puedes entrar.") {
		t.Errorf("no banned reason:\n%s", out)
	}
	if strings.Contains(out, "dispositivo") {
		t.Errorf("the API key reason must not talk about the device:\n%s", out)
	}
}

func TestForbiddenMeAsGuestShowsNoLogin(t *testing.T) {
	f := newFakeSermo(t)
	f.meStatus.Store(403)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, ok := m.top().(*login); ok {
		t.Fatal("a guest must not get the login screen")
	}
}
