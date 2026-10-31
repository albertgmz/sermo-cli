package api

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestServerVersionReadsOnlyThePrefix(t *testing.T) {
	release := make(chan struct{})
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/openapi.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openapi":"3.1.0","info":{"title":"Sermo API","version":"9.8.7",` +
			`"license":{"name":"MIT"}},"paths":{"/a":{`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := c.ServerVersion(ctx)
	if err != nil {
		t.Fatalf("ServerVersion: %v", err)
	}
	if v != "9.8.7" {
		t.Errorf("version = %q, want 9.8.7", v)
	}
}

func TestServerVersionSkipsKeysBeforeInfo(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, `{"x":[1,{"version":"no"}],"info":{"license":{"version":"no"},"version":"1.2.3"}}`)
	})
	v, err := c.ServerVersion(context.Background())
	if err != nil || v != "1.2.3" {
		t.Errorf("ServerVersion = %q, %v, want 1.2.3", v, err)
	}
}

func TestServerVersionFailsWithoutInfo(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, `{"paths":{}}`)
	})
	if _, err := c.ServerVersion(context.Background()); err == nil {
		t.Error("want error")
	}
}

func TestServerVersionReturnsHTTPError(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	_, err := c.ServerVersion(context.Background())
	if !IsNotFound(err) {
		t.Errorf("err = %v, want not found", err)
	}
}

// TestSpecVersionMatchesSavedSpec fails when `make generate` did not run after `make spec`.
func TestSpecVersionMatchesSavedSpec(t *testing.T) {
	f, err := os.Open("../../api/openapi.json")
	if err != nil {
		t.Fatalf("open spec: %v", err)
	}
	defer func() { _ = f.Close() }()
	v, err := infoVersion(f)
	if err != nil {
		t.Fatalf("infoVersion: %v", err)
	}
	if v != SpecVersion {
		t.Errorf("SpecVersion = %q, api/openapi.json has %q", SpecVersion, v)
	}
}
