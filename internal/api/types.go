package api

import (
	"time"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// The types in this file have the same fields as the anonymous success bodies in package gen.
// A facade method converts the gen value to its type, so the TUI can name the type.
// If package gen changes, the conversion does not compile and the type must change too.

// Page is one page of a list that has a page number or a cursor.
// NextCursor is nil after the last page. Total is nil when the request has no page number.
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
	Total      *int    `json:"total"`
}

// CursorPage is one page of a list that has only a cursor. NextCursor is nil after the last page.
type CursorPage[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

// UserMatch is one result of SearchUsers.
type UserMatch struct {
	Id       int
	Staff    bool
	Username string
}

// NewConversation is the success body of ConversationsCreate.
type NewConversation struct {
	Conversation gen.Conversation `json:"conversation"`
	Message      gen.Message      `json:"message"`
}

// Me is the success body of AuthMe.
type Me struct {
	Permissions         gen.GlobalPermissions   `json:"permissions"`
	ResolvedPermissions gen.ResolvedPermissions `json:"resolvedPermissions"`
	Staff               struct {
		ModeratedNodeIds        []int `json:"moderatedNodeIds"`
		ModeratedNodesTruncated bool  `json:"moderatedNodesTruncated"`
		ModeratesEverywhere     bool  `json:"moderatesEverywhere"`
		Panel                   bool  `json:"panel"`
	} `json:"staff"`
	User *gen.SelfUser `json:"user"`
}

// Counters is the success body of AuthCounters.
type Counters struct {
	Conversations    int            `json:"conversations"`
	Notifications    int            `json:"notifications"`
	OpenReports      gen.QueueCount `json:"openReports"`
	PendingApprovals gen.QueueCount `json:"pendingApprovals"`
	PendingSignups   gen.QueueCount `json:"pendingSignups"`
}

// NodeView is the success body of NodesGet.
type NodeView struct {
	Breadcrumbs []struct {
		Id    int                                            `json:"id"`
		Title string                                         `json:"title"`
		Type  gen.NodesGet200JSONResponseBodyBreadcrumbsType `json:"type"`
	} `json:"breadcrumbs"`
	Node                gen.Node                `json:"node"`
	ResolvedPermissions gen.ResolvedPermissions `json:"resolvedPermissions"`
	Seo                 struct {
		Canonical   string                                   `json:"canonical"`
		Description string                                   `json:"description"`
		JsonLd      []interface{}                            `json:"jsonLd"`
		Redirect    bool                                     `json:"redirect"`
		Robots      gen.NodesGet200JSONResponseBodySeoRobots `json:"robots"`
		Title       string                                   `json:"title"`
	} `json:"seo"`
	Watching *struct {
		Email bool                                        `json:"email"`
		Mode  gen.NodesGet200JSONResponseBodyWatchingMode `json:"mode"`
	} `json:"watching"`
}

// ThreadList is the success body of ThreadsList.
type ThreadList struct {
	Items      []gen.Thread `json:"items"`
	NextCursor *string      `json:"nextCursor"`
	Sticky     []gen.Thread `json:"sticky"`
	Total      *int         `json:"total"`
}

// NewThread is the success body of ThreadsCreate.
type NewThread struct {
	Post   gen.PostDetail `json:"post"`
	Thread gen.Thread     `json:"thread"`
}

// NotificationList is the success body of NotificationsList.
type NotificationList struct {
	Items       []gen.Notification `json:"items"`
	NextCursor  *string            `json:"nextCursor"`
	Total       *int               `json:"total"`
	UnreadCount int                `json:"unreadCount"`
}

// ThreadView is the success body of ThreadsGet.
type ThreadView struct {
	Featured *gen.FeatureKind `json:"featured"`
	Node     struct {
		Id    int    `json:"id"`
		Title string `json:"title"`
	} `json:"node"`
	Permissions struct {
		CanDelete      bool `json:"canDelete"`
		CanEditTags    bool `json:"canEditTags"`
		CanEditTitle   bool `json:"canEditTitle"`
		CanHardDelete  bool `json:"canHardDelete"`
		CanLock        bool `json:"canLock"`
		CanModerate    bool `json:"canModerate"`
		CanRemoveCover bool `json:"canRemoveCover"`
		CanReply       bool `json:"canReply"`
		CanReport      bool `json:"canReport"`
		CanWatch       bool `json:"canWatch"`
	} `json:"permissions"`
	Poll struct {
		CanAdd bool `json:"canAdd"`
		Exists bool `json:"exists"`
	} `json:"poll"`
	ResolvedPermissions gen.ResolvedPermissions `json:"resolvedPermissions"`
	Seo                 struct {
		Canonical   string                                     `json:"canonical"`
		Description string                                     `json:"description"`
		JsonLd      []interface{}                              `json:"jsonLd"`
		Redirect    bool                                       `json:"redirect"`
		Robots      gen.ThreadsGet200JSONResponseBodySeoRobots `json:"robots"`
		Title       string                                     `json:"title"`
	} `json:"seo"`
	SeoOverride *struct {
		Description *string `json:"description"`
		Title       *string `json:"title"`
	} `json:"seoOverride"`
	Tags     []string   `json:"tags"`
	Thread   gen.Thread `json:"thread"`
	Watching *struct {
		Email bool `json:"email"`
	} `json:"watching"`
}

// PollView is the success body of PollsGet.
type PollView struct {
	CanCreate bool      `json:"canCreate"`
	Poll      *gen.Poll `json:"poll"`
}

// PostList is the success body of PostsList.
type PostList struct {
	AfterPostId *int       `json:"afterPostId,omitempty"`
	Items       []gen.Post `json:"items"`
	NextCursor  *string    `json:"nextCursor"`
}

// UnreadPost is the success body of ThreadsLocateUnread.
type UnreadPost struct {
	Position int `json:"position"`
	PostId   int `json:"postId"`
	ThreadId int `json:"threadId"`
}

// Profile is the success body of ProfilesGet.
type Profile struct {
	About  string `json:"about"`
	Avatar *struct {
		FileId int    `json:"fileId"`
		Url    string `json:"url"`
	} `json:"avatar"`
	AwardSlots int `json:"awardSlots"`
	AwardXp    int `json:"awardXp"`
	Birthday   *struct {
		Day   int `json:"day"`
		Month int `json:"month"`
	} `json:"birthday"`
	CanPostOnWall bool           `json:"canPostOnWall"`
	Card          gen.MemberCard `json:"card"`
	Cover         *struct {
		FileId int    `json:"fileId"`
		Url    string `json:"url"`
	} `json:"cover"`

	CreatedAt      time.Time         `json:"createdAt"`
	Credits        int               `json:"credits"`
	CustomTitle    *string           `json:"customTitle"`
	Device         *gen.MemberDevice `json:"device"`
	DisplayGroup   *gen.DisplayGroup `json:"displayGroup"`
	FollowerCount  int               `json:"followerCount"`
	FollowingCount int               `json:"followingCount"`
	GroupTitle     string            `json:"groupTitle"`
	Id             int               `json:"id"`
	IsAdult        bool              `json:"isAdult"`
	LastActivityAt *time.Time        `json:"lastActivityAt"`
	Level          struct {
		CurrentLevelXp int `json:"currentLevelXp"`
		Level          int `json:"level"`
		NextLevelXp    int `json:"nextLevelXp"`
	} `json:"level"`
	Location      string `json:"location"`
	PostCount     int    `json:"postCount"`
	ReactionScore int    `json:"reactionScore"`
	ReferralCount int    `json:"referralCount"`
	Seo           struct {
		Canonical   string                                      `json:"canonical"`
		Description string                                      `json:"description"`
		JsonLd      []interface{}                               `json:"jsonLd"`
		Redirect    bool                                        `json:"redirect"`
		Robots      gen.ProfilesGet200JSONResponseBodySeoRobots `json:"robots"`
		Title       string                                      `json:"title"`
	} `json:"seo"`
	SignatureHtml *string `json:"signatureHtml"`
	Socials       struct {
		Discord  string `json:"discord"`
		Telegram string `json:"telegram"`
		X        string `json:"x"`
	} `json:"socials"`
	Username string `json:"username"`
	Viewer   struct {
		IsBlocking  bool `json:"isBlocking"`
		IsFollowing bool `json:"isFollowing"`
	} `json:"viewer"`
	Website string `json:"website"`
}
