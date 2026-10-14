package domain

import (
	"fmt"
	"strconv"
	"time"
)

// StatusLine is the part of Claude Code's status-line JSON claude-switch reads.
type StatusLine struct {
	SessionID  string      `json:"session_id"`
	RateLimits *RateLimits `json:"rate_limits"`
}

// RateLimits holds the subscription usage windows.
type RateLimits struct {
	FiveHour *Window `json:"five_hour"`
	SevenDay *Window `json:"seven_day"`
}

// Window is one usage window. ResetsAt is Unix seconds.
type Window struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

// RecordReport stores the active account's usage from a status line and tells
// whether it is time to switch.
//
// A session keeps showing its last response's numbers until the next response.
// Right after a switch those still belong to the old account, so a report that
// repeats the session's last signature is ignored.
func (s *State) RecordReport(in StatusLine, threshold int, now time.Time) (shouldSwitch bool) {
	if s.ActiveName() == "" || in.RateLimits == nil || in.RateLimits.FiveHour == nil {
		return false
	}
	five := in.RateLimits.FiveHour
	seven := in.RateLimits.SevenDay
	if seven == nil {
		seven = &Window{}
	}
	f, w := pct(five.UsedPercentage), pct(seven.UsedPercentage)
	sig := fmt.Sprintf("%s|%s|%s|%s", num(f), opt(five.ResetsAt), num(w), opt(seven.ResetsAt))

	sid := in.SessionID
	if sid == "" {
		sid = "none"
	}
	if s.Sessions[sid].Sig == sig {
		return false
	}
	n := unixSeconds(now)
	s.Sessions[sid] = Session{Sig: sig, At: n}
	for k, v := range s.Sessions {
		if v.At <= n-sessionTTL.Seconds() {
			delete(s.Sessions, k)
		}
	}
	s.Usage[s.ActiveName()] = Usage{Five: &f, FiveReset: five.ResetsAt, Seven: &w, SevenReset: seven.ResetsAt, At: n}
	t := float64(threshold)
	return f >= t || w >= t
}

func pct(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func opt(f *float64) string {
	if f == nil {
		return "null"
	}
	return num(*f)
}
