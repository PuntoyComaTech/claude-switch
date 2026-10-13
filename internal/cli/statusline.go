package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/PuntoyComaTech/claude-switch/internal/domain"
)

// statusInput is the status-line JSON with the fields the built-in status
// line shows on top of what report reads.
type statusInput struct {
	domain.StatusLine
	Model *struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
}

// statusText renders "account · model · 5h 40% · 7d 12%".
func statusText(account string, in statusInput) string {
	if account == "" {
		account = "?"
	}
	parts := []string{account}
	if in.Model != nil && in.Model.DisplayName != "" {
		parts = append(parts, in.Model.DisplayName)
	}
	if r := in.RateLimits; r != nil {
		if p := windowPct(r.FiveHour); p != "" {
			parts = append(parts, "5h "+p)
		}
		if p := windowPct(r.SevenDay); p != "" {
			parts = append(parts, "7d "+p)
		}
	}
	return strings.Join(parts, " · ")
}

func windowPct(w *domain.Window) string {
	if w == nil || w.UsedPercentage == nil {
		return ""
	}
	return fmt.Sprintf("%d%%", int(math.Floor(*w.UsedPercentage)))
}
