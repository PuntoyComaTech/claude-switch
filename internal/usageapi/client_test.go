package usageapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/usageapi"
)

func client(url string) usageapi.Client {
	return usageapi.Client{
		URL: url, HTTP: http.DefaultClient, UserAgent: "claude-switch/test",
		Now: func() time.Time { return time.Unix(100, 0) },
	}
}

func TestReadParsesWindows(t *testing.T) {
	var gotAuth, gotUA, gotBeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotUA, gotBeta = r.Header.Get("Authorization"), r.Header.Get("User-Agent"), r.Header.Get("anthropic-beta")
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":99.0,"resets_at":"2099-01-01T10:00:00.123456+00:00"},
			"seven_day":{"utilization":20,"resets_at":"2099-01-05T10:00:00Z"}}`))
	}))
	defer srv.Close()

	u, err := client(srv.URL).Read(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" || gotUA != "claude-switch/test" || gotBeta != "oauth-2025-04-20" {
		t.Errorf("headers: %q %q %q", gotAuth, gotUA, gotBeta)
	}
	if *u.Five != 99 || *u.FiveReset != 4070944800 || *u.Seven != 20 || *u.SevenReset != 4071290400 {
		t.Errorf("usage: %+v", u)
	}
	if u.At != 100 || u.Source != "api" {
		t.Errorf("metadata: %+v", u)
	}
}

func TestReadMissingWindowStaysNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"five_hour":null,"seven_day":{"utilization":5,"resets_at":"bogus"}}`))
	}))
	defer srv.Close()
	u, err := client(srv.URL).Read(context.Background(), "t")
	if err != nil || u.Five != nil || *u.Seven != 5 || u.SevenReset != nil {
		t.Fatalf("got %+v, %v", u, err)
	}
}

func TestReadErrors(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"http status": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		"bad json":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) },
	} {
		srv := httptest.NewServer(h)
		if _, err := client(srv.URL).Read(context.Background(), "t"); err == nil {
			t.Errorf("%s: want error", name)
		}
		srv.Close()
	}
}
