package api

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestSetReactionSendsPutWithType(t *testing.T) {
	var method, path, body, ctype string
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, path, body, ctype = r.Method, r.URL.Path, string(b), r.Header.Get("Content-Type")
		writeJSON(w, http.StatusOK, `{"counts":{"3":2},"mine":3,"total":2}`)
	})
	sum, err := c.SetReaction(context.Background(), ContentPost, 42, 3)
	if err != nil {
		t.Fatalf("SetReaction: %v", err)
	}
	if method != http.MethodPut || path != "/api/v1/reactions/post/42" {
		t.Errorf("request = %s %s", method, path)
	}
	if body != `{"reactionTypeId":3}` || ctype != "application/json" {
		t.Errorf("body = %q, type = %q", body, ctype)
	}
	if sum.Total != 2 || sum.Mine == nil || *sum.Mine != 3 || sum.Counts["3"] != 2 {
		t.Errorf("summary = %+v", sum)
	}
}

func TestRemoveReactionSendsDelete(t *testing.T) {
	var method, path string
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		writeJSON(w, http.StatusOK, `{"counts":{},"mine":null,"total":0}`)
	})
	sum, err := c.RemoveReaction(context.Background(), ContentMessage, 9)
	if err != nil {
		t.Fatalf("RemoveReaction: %v", err)
	}
	if method != http.MethodDelete || path != "/api/v1/reactions/conversation_message/9" {
		t.Errorf("request = %s %s", method, path)
	}
	if sum.Mine != nil || sum.Total != 0 {
		t.Errorf("summary = %+v", sum)
	}
}

func TestReactionsListsUsers(t *testing.T) {
	var query string
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		writeJSON(w, http.StatusOK, `{"items":[{"user":{"id":5,"username":"pepe"},"reactionTypeId":1,`+
			`"createdAt":"2026-10-31T10:00:00Z"}],"nextCursor":"abc"}`)
	})
	page, err := c.Reactions(context.Background(), ContentProfilePost, 8, "cur", 10)
	if err != nil {
		t.Fatalf("Reactions: %v", err)
	}
	if query != "cursor=cur&limit=10" {
		t.Errorf("query = %q", query)
	}
	if len(page.Items) != 1 || page.Items[0].User.Username != "pepe" || page.NextCursor == nil || *page.NextCursor != "abc" {
		t.Errorf("page = %+v", page)
	}
}

func TestReactionErrorUsesEnvelope(t *testing.T) {
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusForbidden, `{"error":{"code":"forbidden","message":"No"}}`)
	})
	_, err := c.SetReaction(context.Background(), ContentPost, 1, 1)
	if !IsForbidden(err) {
		t.Errorf("err = %v, want forbidden", err)
	}
}
