package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRefusesBadSettings(t *testing.T) {
	for _, tc := range []struct{ url, token, want string }{
		{"monitor.example.com", "mnp_x", "http(s)://host"},
		{"ftp://monitor.example.com", "mnp_x", "http(s)://host"},
		{"https://monitor.example.com", "mns_session", "workspace token"},
	} {
		if _, err := New(tc.url, tc.token); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("New(%q, %q) = %v, want an error with %q", tc.url, tc.token, err, tc.want)
		}
	}
}

func TestRequestsAndErrors(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/workspace/checks":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"c1","name":"Website","kind":"http","interval_seconds":60,"channel_ids":[]}`))
		case "/v1/workspace/checks/gone":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no such check"}}`))
		case "/v1/workspace/checks/bad":
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid","message":"name must be 1 to 100 characters"}}`))
		case "/v1/workspace/maintenance":
			_, _ = w.Write([]byte(`[{"id":"m1","title":"Upgrade","starts_at":"2026-12-01T02:00:00Z","ends_at":"2026-12-01T03:00:00Z","check_ids":["c1"]}]`))
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL+"/", "mnp_secret", WithUserAgent("test/1"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	ids := []string{}
	ch, err := c.CreateCheck(ctx, CheckWrite{Name: "Website", Kind: "http", Target: "https://example.com/", ChannelIDs: &ids})
	if err != nil || ch.ID != "c1" || ch.IntervalSeconds != 60 {
		t.Fatalf("create: %+v %v", ch, err)
	}
	if got.Method != "POST" || got.Header.Get("Authorization") != "Bearer mnp_secret" || got.Header.Get("User-Agent") != "test/1" ||
		got.Header.Get("Content-Type") != "application/json" {
		t.Errorf("request %s %v", got.Method, got.Header)
	}
	var sent map[string]any
	_ = json.Unmarshal(body, &sent)
	// An empty channel list is sent (it clears the channels); unset numbers are left to the server.
	if v, ok := sent["channel_ids"]; !ok || len(v.([]any)) != 0 {
		t.Errorf("channel_ids not sent as []: %s", body)
	}
	if _, ok := sent["interval_seconds"]; ok {
		t.Errorf("unset interval sent: %s", body)
	}

	_, err = c.GetCheck(ctx, "gone")
	if !IsNotFound(err) {
		t.Errorf("404 not recognised: %v", err)
	}
	_, err = c.UpdateCheck(ctx, "bad", CheckWrite{})
	if err == nil || IsNotFound(err) || !strings.Contains(err.Error(), "name must be 1 to 100 characters") {
		t.Errorf("422: %v", err)
	}
	_, _ = c.GetCheck(ctx, "../../admin")
	if p := got.URL.EscapedPath(); p != "/v1/workspace/checks/..%2F..%2Fadmin" {
		t.Errorf("an id was not kept to one path segment: %s", p)
	}

	m, err := c.GetMaintenance(ctx, "m1")
	if err != nil || m.Title != "Upgrade" || !m.StartsAt.Equal(time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)) {
		t.Errorf("maintenance: %+v %v", m, err)
	}
	if _, err := c.GetMaintenance(ctx, "m2"); !IsNotFound(err) {
		t.Errorf("missing maintenance: %v", err)
	}
}
