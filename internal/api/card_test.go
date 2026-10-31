package api

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

const profileJSON = `{"id":5,"username":"kz0","groupTitle":"Registered","displayGroup":{"id":3,"title":"Veterans",` +
	`"userTitle":"Veterano","badge":""},"createdAt":"2026-03-01T10:00:00Z","postCount":812,"reactionScore":214,` +
	`"customTitle":"Hacker","signatureHtml":"<p>firma</p>"}`

func TestAuthorCardReadsProfileAndCaches(t *testing.T) {
	cnt := &counter{}
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		cnt.add(r)
		writeJSON(w, http.StatusOK, profileJSON)
	})
	ctx := context.Background()
	card, err := c.AuthorCard(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := Card{GroupTitle: "Veterano", CustomTitle: "Hacker", PostCount: 812, ReactionScore: 214,
		JoinedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), SignatureHTML: "<p>firma</p>"}
	if *card != want {
		t.Errorf("card = %+v, want %+v", *card, want)
	}
	if _, err := c.AuthorCard(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if n := cnt.get("GET /api/v1/users/5"); n != 1 {
		t.Errorf("profile requests = %d, want 1 (cache hit)", n)
	}
}

func TestAuthorCardGroupTitleWithoutDisplayGroup(t *testing.T) {
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"id":6,"groupTitle":"Miembro","displayGroup":null,"createdAt":"2026-01-01T00:00:00Z",`+
			`"customTitle":null,"signatureHtml":null}`)
	})
	card, err := c.AuthorCard(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	if card.GroupTitle != "Miembro" || card.CustomTitle != "" || card.SignatureHTML != "" {
		t.Errorf("card = %+v", *card)
	}
}

func TestAuthorCardSharesRequestInFlight(t *testing.T) {
	cnt := &counter{}
	release := make(chan struct{})
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		cnt.add(r)
		<-release
		writeJSON(w, http.StatusOK, profileJSON)
	})
	var wg sync.WaitGroup
	cards := make([]*Card, 2)
	for i := range cards {
		wg.Go(func() {
			card, err := c.AuthorCard(context.Background(), 5)
			if err != nil {
				t.Error(err)
			}
			cards[i] = card
		})
	}
	// Give both calls time to start before the server answers.
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := cnt.get("GET /api/v1/users/5"); n != 1 {
		t.Errorf("profile requests = %d, want 1", n)
	}
	if cards[0] == nil || cards[1] == nil || cards[0].PostCount != 812 || cards[1].PostCount != 812 {
		t.Errorf("cards = %+v, %+v", cards[0], cards[1])
	}
}

func TestAuthorCardGuestIsNil(t *testing.T) {
	cnt := &counter{}
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) { cnt.add(r) })
	card, err := c.AuthorCard(context.Background(), 0)
	if card != nil || err != nil {
		t.Errorf("AuthorCard(0) = %v, %v, want nil, nil", card, err)
	}
	if n := cnt.get("GET /api/v1/users/0"); n != 0 {
		t.Errorf("guest made %d requests", n)
	}
}

func TestAuthorCardErrorIsNotCached(t *testing.T) {
	cnt := &counter{}
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		cnt.add(r)
		if cnt.get("GET /api/v1/users/5") == 1 {
			writeJSON(w, http.StatusNotFound, `{"error":{"code":"not_found","message":"x"}}`)
			return
		}
		writeJSON(w, http.StatusOK, profileJSON)
	})
	ctx := context.Background()
	if _, err := c.AuthorCard(ctx, 5); !IsNotFound(err) {
		t.Fatalf("first call err = %v, want 404", err)
	}
	if card, err := c.AuthorCard(ctx, 5); err != nil || card.PostCount != 812 {
		t.Errorf("second call = %+v, %v", card, err)
	}
}

func statsServer(t *testing.T) (*Client, *counter) {
	t.Helper()
	cnt := &counter{}
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		cnt.add(r)
		switch r.URL.Path {
		case "/api/v1/online":
			writeJSON(w, http.StatusOK, `{"items":[{"type":"member","card":{"id":5,"username":"kz0"},"robot":null,`+
				`"ip":null,"activity":{}}],"page":1,"perPage":25,"total":37}`)
		case "/api/v1/site/stats":
			writeJSON(w, http.StatusOK, `{"threads":10,"posts":200,"members":4182,"latestMember":null}`)
		}
	})
	return c, cnt
}

func TestOnlineList(t *testing.T) {
	c, _ := statsServer(t)
	on, err := c.OnlineList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if on.Total != 37 || len(on.Items) != 1 || on.Items[0].Card.Username != "kz0" {
		t.Errorf("online = %+v", on)
	}
}

func TestSiteStatsOncePerSession(t *testing.T) {
	c, cnt := statsServer(t)
	for range 2 {
		st, err := c.SiteStats(context.Background())
		if err != nil || st.Members != 4182 || st.Posts != 200 || st.Threads != 10 {
			t.Errorf("stats = %+v, %v", st, err)
		}
	}
	if n := cnt.get("GET /api/v1/site/stats"); n != 1 {
		t.Errorf("stats requests = %d, want 1", n)
	}
}
