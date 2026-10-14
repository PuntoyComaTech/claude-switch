// Package usageapi reads a subscription's usage windows from Anthropic's
// OAuth usage endpoint, the same data Claude Code shows in /usage.
package usageapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// DefaultURL is the OAuth usage endpoint.
const DefaultURL = "https://api.anthropic.com/api/oauth/usage"

// Client implements app.UsageReader.
type Client struct {
	URL       string
	HTTP      *http.Client
	UserAgent string
	Now       func() time.Time
}

var _ app.UsageReader = Client{}

type window struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type response struct {
	FiveHour *window `json:"five_hour"`
	SevenDay *window `json:"seven_day"`
}

// Read implements app.UsageReader. It identifies itself honestly through
// UserAgent.
func (c Client) Read(ctx context.Context, token string) (domain.Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, http.NoBody)
	if err != nil {
		return domain.Usage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return domain.Usage{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.Usage{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var r response
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return domain.Usage{}, fmt.Errorf("unexpected response: %w", err)
	}
	u := domain.Usage{At: float64(c.Now().UnixNano()) / 1e9, Source: "api"}
	if r.FiveHour != nil {
		u.Five, u.FiveReset = r.FiveHour.Utilization, epoch(r.FiveHour.ResetsAt)
	}
	if r.SevenDay != nil {
		u.Seven, u.SevenReset = r.SevenDay.Utilization, epoch(r.SevenDay.ResetsAt)
	}
	return u, nil
}

// epoch turns an RFC 3339 time into Unix seconds; nil or invalid stays nil.
func epoch(s *string) *float64 {
	if s == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		return nil
	}
	v := float64(t.Unix())
	return &v
}
