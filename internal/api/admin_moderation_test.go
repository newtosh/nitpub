package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func jsonBody(s string) io.Reader { return strings.NewReader(s) }

func newModerationTestHandler(t *testing.T) (*Handler, *moderation.Service, *Auth, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, mod, "", false)
	return h, mod, auth, sid
}

func TestAdminListPendingRepliesRequiresAuth(t *testing.T) {
	h, _, _, _ := newModerationTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/replies", nil)
	rec := httptest.NewRecorder()
	h.AdminListPendingReplies(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAdminListPendingReplies(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "hi", Status: moderation.StatusPending}); err != nil {
		t.Fatal(err)
	}
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a2", PostSlug: "p", Actor: "x", Content: "approved-already", Status: moderation.StatusApproved}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/replies", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminListPendingReplies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var replies []moderation.Reply
	if err := json.NewDecoder(rec.Body).Decode(&replies); err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || replies[0].Content != "hi" {
		t.Fatalf("expected only the pending reply, got %+v", replies)
	}
}

func TestAdminApproveReplyRequiresAuth(t *testing.T) {
	h, _, _, _ := newModerationTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/replies/whatever/approve", nil)
	rec := httptest.NewRecorder()
	h.AdminApproveReply(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAdminApproveReply(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "hi", Status: moderation.StatusPending}); err != nil {
		t.Fatal(err)
	}
	all, err := mod.RepliesForPost("p")
	if err != nil || len(all) != 1 {
		t.Fatalf("setup: %v %+v", err, all)
	}
	key := all[0].Key

	req := httptest.NewRequest(http.MethodPost, "/api/admin/replies/"+key+"/approve", nil)
	req.SetPathValue("id", key)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminApproveReply(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	approved, err := mod.ApprovedRepliesForPost("p")
	if err != nil || len(approved) != 1 {
		t.Fatalf("expected the reply to be approved, got %v %+v", err, approved)
	}
}

func TestAdminRejectReply(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "hi", Status: moderation.StatusPending}); err != nil {
		t.Fatal(err)
	}
	all, _ := mod.RepliesForPost("p")
	key := all[0].Key

	req := httptest.NewRequest(http.MethodPost, "/api/admin/replies/"+key+"/reject", nil)
	req.SetPathValue("id", key)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminRejectReply(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	approved, err := mod.ApprovedRepliesForPost("p")
	if err != nil || len(approved) != 0 {
		t.Fatalf("expected no approved replies after reject, got %v %+v", err, approved)
	}
}

func TestAdminApproveRejectNonexistentReply404s(t *testing.T) {
	h, _, _, sid := newModerationTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/replies/does-not-exist/approve", nil)
	req.SetPathValue("id", "does-not-exist")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminApproveReply(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminTrustedBlockedActorsRequireAuth(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		path    string
		handler func(h *Handler) http.HandlerFunc
	}{
		{"list trusted", http.MethodGet, "/api/admin/moderation/trusted", func(h *Handler) http.HandlerFunc { return h.AdminListTrustedActors }},
		{"add trusted", http.MethodPost, "/api/admin/moderation/trusted", func(h *Handler) http.HandlerFunc { return h.AdminAddTrustedActor }},
		{"remove trusted", http.MethodDelete, "/api/admin/moderation/trusted/x", func(h *Handler) http.HandlerFunc { return h.AdminRemoveTrustedActor }},
		{"list blocked", http.MethodGet, "/api/admin/moderation/blocked", func(h *Handler) http.HandlerFunc { return h.AdminListBlockedActors }},
		{"add blocked", http.MethodPost, "/api/admin/moderation/blocked", func(h *Handler) http.HandlerFunc { return h.AdminAddBlockedActor }},
		{"remove blocked", http.MethodDelete, "/api/admin/moderation/blocked/x", func(h *Handler) http.HandlerFunc { return h.AdminRemoveBlockedActor }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _, _ := newModerationTestHandler(t)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			tc.handler(h)(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestAdminTrustedActorsAddListRemove(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	actor := "https://remote.example/users/alice"
	body := `{"actor":"` + actor + `"}`

	addReq := httptest.NewRequest(http.MethodPost, "/api/admin/moderation/trusted", jsonBody(body))
	addReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	addRec := httptest.NewRecorder()
	h.AdminAddTrustedActor(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status = %d body=%s", addRec.Code, addRec.Body.String())
	}
	trusted, err := mod.IsTrusted(actor)
	if err != nil || !trusted {
		t.Fatalf("expected trusted after add, got %v %v", trusted, err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/moderation/trusted", nil)
	listReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	listRec := httptest.NewRecorder()
	h.AdminListTrustedActors(listRec, listReq)
	var list []string
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != actor {
		t.Fatalf("expected trusted list to contain the actor, got %+v", list)
	}

	remReq := httptest.NewRequest(http.MethodDelete, "/api/admin/moderation/trusted/"+actor, nil)
	remReq.SetPathValue("actor", actor)
	remReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	remRec := httptest.NewRecorder()
	h.AdminRemoveTrustedActor(remRec, remReq)
	if remRec.Code != http.StatusOK {
		t.Fatalf("remove status = %d body=%s", remRec.Code, remRec.Body.String())
	}
	trusted, err = mod.IsTrusted(actor)
	if err != nil || trusted {
		t.Fatalf("expected not trusted after remove, got %v %v", trusted, err)
	}
}

func TestAdminBlockedActorsAddListRemove(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	actor := "https://remote.example/users/mallory"
	body := `{"actor":"` + actor + `"}`

	addReq := httptest.NewRequest(http.MethodPost, "/api/admin/moderation/blocked", jsonBody(body))
	addReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	addRec := httptest.NewRecorder()
	h.AdminAddBlockedActor(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status = %d body=%s", addRec.Code, addRec.Body.String())
	}
	blocked, err := mod.IsBlocked(actor)
	if err != nil || !blocked {
		t.Fatalf("expected blocked after add, got %v %v", blocked, err)
	}

	remReq := httptest.NewRequest(http.MethodDelete, "/api/admin/moderation/blocked/"+actor, nil)
	remReq.SetPathValue("actor", actor)
	remReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	remRec := httptest.NewRecorder()
	h.AdminRemoveBlockedActor(remRec, remReq)
	if remRec.Code != http.StatusOK {
		t.Fatalf("remove status = %d body=%s", remRec.Code, remRec.Body.String())
	}
	blocked, err = mod.IsBlocked(actor)
	if err != nil || blocked {
		t.Fatalf("expected not blocked after remove, got %v %v", blocked, err)
	}
}

func TestAdminBlockingActorDoesNotRetroactivelyRejectExistingReplies(t *testing.T) {
	h, mod, _, sid := newModerationTestHandler(t)
	actor := "https://remote.example/users/mallory"
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: "p", Actor: actor, Content: "hi", Status: moderation.StatusPending}); err != nil {
		t.Fatal(err)
	}

	body := `{"actor":"` + actor + `"}`
	addReq := httptest.NewRequest(http.MethodPost, "/api/admin/moderation/blocked", jsonBody(body))
	addReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	addRec := httptest.NewRecorder()
	h.AdminAddBlockedActor(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("add status = %d", addRec.Code)
	}

	all, err := mod.RepliesForPost("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Status != moderation.StatusPending {
		t.Fatalf("blocking must not retroactively change existing reply status, got %+v", all)
	}
}
