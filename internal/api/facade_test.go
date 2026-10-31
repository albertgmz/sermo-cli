package api

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestThreadsSendsPageOptions(t *testing.T) {
	var path, query string
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		writeJSON(w, http.StatusOK, `{"items":[{"id":11,"title":"Hola"}],"sticky":[],"nextCursor":"n2","total":null}`)
	})
	list, err := c.Threads(context.Background(), 3, 0, PageOpts{Cursor: "n1", Limit: 20})
	if err != nil {
		t.Fatalf("Threads: %v", err)
	}
	if path != "/api/v1/nodes/3/threads" || query != "cursor=n1&limit=20" {
		t.Errorf("request = %s?%s", path, query)
	}
	if len(list.Items) != 1 || list.Items[0].Title != "Hola" || *list.NextCursor != "n2" {
		t.Errorf("list = %+v", list)
	}
}

func TestReplySendsBody(t *testing.T) {
	var method, path, body string
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, path, body = r.Method, r.URL.Path, string(b)
		writeJSON(w, http.StatusCreated, `{"id":99,"threadId":4,"bodyHtml":"<p>x</p>","author":{"kind":"member","id":1,"username":"a"}}`)
	})
	p, err := c.Reply(context.Background(), 4, "[B]x[/B]")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if method != http.MethodPost || path != "/api/v1/threads/4/posts" || body != `{"body":"[B]x[/B]"}` {
		t.Errorf("request = %s %s %s", method, path, body)
	}
	if p.Id != 99 || AuthorName(p.Author) != "a" {
		t.Errorf("post = %+v", p)
	}
}
