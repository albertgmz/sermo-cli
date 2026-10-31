package api

import (
	"context"
	"net/http"
	"testing"
)

func TestSpoilerSendsQuery(t *testing.T) {
	var path, query string
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		writeJSON(w, http.StatusOK, `{"html":"<p>secreto</p>"}`)
	})
	html, err := c.Spoiler(context.Background(), ContentPost, 77, 2)
	if err != nil {
		t.Fatalf("Spoiler: %v", err)
	}
	if path != "/api/v1/spoilers" || query != "contentType=post&contentId=77&position=2" {
		t.Errorf("request = %s?%s", path, query)
	}
	if html != "<p>secreto</p>" {
		t.Errorf("html = %q", html)
	}
}

func TestSpoilerNotFound(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, `{"error":{"code":"not_found","message":"No existe."}}`)
	})
	if _, err := c.Spoiler(context.Background(), ContentPost, 77, 9); !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
}
