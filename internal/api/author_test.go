package api

import (
	"encoding/json"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

func TestAuthorHelpers(t *testing.T) {
	tests := []struct {
		raw    string
		name   string
		id     int
		member bool
	}{
		{`{"kind":"member","id":7,"username":"yoel"}`, "yoel", 7, true},
		{`{"kind":"guest","name":"Visitante"}`, "Visitante", 0, false},
		{`{"kind":"guest","name":null}`, "", 0, false},
	}
	for _, tc := range tests {
		var a gen.ContentAuthor
		if err := json.Unmarshal([]byte(tc.raw), &a); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if got := AuthorName(a); got != tc.name {
			t.Errorf("AuthorName(%s) = %q, want %q", tc.raw, got, tc.name)
		}
		if got := AuthorID(a); got != tc.id {
			t.Errorf("AuthorID(%s) = %d, want %d", tc.raw, got, tc.id)
		}
		if got := IsMember(a); got != tc.member {
			t.Errorf("IsMember(%s) = %v, want %v", tc.raw, got, tc.member)
		}
	}
}

func TestPreviewTargets(t *testing.T) {
	tests := []struct {
		target gen.PreviewTarget
		want   string
	}{
		{NewThreadTarget(3), `{"nodeId":3,"type":"thread"}`},
		{ReplyTarget(4), `{"threadId":4,"type":"reply"}`},
		{MessageTarget(5), `{"conversationId":5,"type":"message"}`},
		{MessageTarget(0), `{"type":"message"}`},
	}
	for _, tc := range tests {
		b, err := json.Marshal(tc.target)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(b) != tc.want {
			t.Errorf("target = %s, want %s", b, tc.want)
		}
	}
}
