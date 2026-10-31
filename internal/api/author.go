package api

import "github.com/albertgmz/sermo-cli/internal/api/gen"

// AuthorName returns the username of a member or the name of a guest.
// It returns "" for a guest without a name or for data it cannot read.
func AuthorName(a gen.ContentAuthor) string {
	if m, ok := member(a); ok {
		return m.Username
	}
	g, err := a.AsGuestAuthor()
	if err != nil || g.Name == nil {
		return ""
	}
	return *g.Name
}

// AuthorID returns the user ID of a member author, or 0 for a guest.
func AuthorID(a gen.ContentAuthor) int {
	m, _ := member(a)
	return m.Id
}

// IsMember reports whether the author is a member and not a guest.
func IsMember(a gen.ContentAuthor) bool {
	_, ok := member(a)
	return ok
}

func member(a gen.ContentAuthor) (gen.MemberAuthor, bool) {
	m, err := a.AsMemberAuthor()
	if err != nil || m.Kind != gen.Member {
		return gen.MemberAuthor{}, false
	}
	return m, true
}

// NewThreadTarget is the Preview target for a new thread in the node nodeID.
func NewThreadTarget(nodeID int) gen.PreviewTarget {
	var t gen.PreviewTarget
	_ = t.FromPreviewTarget0(gen.PreviewTarget0{NodeId: nodeID, Type: gen.PreviewTarget0TypeThread})
	return t
}

// ReplyTarget is the Preview target for a reply in the thread threadID.
func ReplyTarget(threadID int) gen.PreviewTarget {
	var t gen.PreviewTarget
	_ = t.FromPreviewTarget1(gen.PreviewTarget1{ThreadId: threadID, Type: gen.Reply})
	return t
}

// MessageTarget is the Preview target for a conversation message.
// Use 0 for a new conversation.
func MessageTarget(conversationID int) gen.PreviewTarget {
	v := gen.PreviewTarget2{Type: gen.PreviewTarget2TypeMessage}
	if conversationID != 0 {
		v.ConversationId = &conversationID
	}
	var t gen.PreviewTarget
	_ = t.FromPreviewTarget2(v)
	return t
}
