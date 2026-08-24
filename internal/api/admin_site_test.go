package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/sitecontent"
)

func TestAdminPutManifestUpdatesPublicSite(t *testing.T) {
	h, sid := testSiteHandler(t)
	manifest := sitecontent.Manifest{
		Home:    sitecontent.HomeConfig{RecentCount: 3},
		Archive: sitecontent.ArchiveConfig{Mode: "pagination", PageSize: 15},
		Search:  sitecontent.SearchConfig{Enabled: true},
		Nav: []sitecontent.NavItem{
			{Label: "About", Path: "/about", Icon: "user"},
		},
		Pages: []sitecontent.PageRef{
			{Path: "/about", Type: "markdown", File: "pages/about.md"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/site/manifest", bytes.NewReader(data))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminPutManifest(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status = %d body=%s", rec.Code, rec.Body.String())
	}

	pub := httptest.NewRequest(http.MethodGet, "/api/site", nil)
	pubRec := httptest.NewRecorder()
	h.ServeSite(pubRec, pub)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("get status = %d", pubRec.Code)
	}
	var body struct {
		Home struct {
			RecentCount int `json:"recent_count"`
		} `json:"home"`
	}
	if err := json.NewDecoder(pubRec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Home.RecentCount != 3 {
		t.Fatalf("recent_count = %d", body.Home.RecentCount)
	}
}

func TestAdminGetSiteRequiresAuth(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/site", nil)
	rec := httptest.NewRecorder()
	h.AdminGetSite(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}
