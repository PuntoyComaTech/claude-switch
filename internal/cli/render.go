package cli

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
)

type palette struct{ bold, dim, red, yellow, green, reset string }

func colors(on bool) palette {
	if !on {
		return palette{}
	}
	return palette{"\x1b[1m", "\x1b[2m", "\x1b[31m", "\x1b[33m", "\x1b[32m", "\x1b[0m"}
}

// renderList prints the account table of `list`.
func renderList(w io.Writer, v app.ListView, now time.Time, color bool) {
	c := colors(color)
	if len(v.Accounts) == 0 {
		_, _ = fmt.Fprintln(w, "no accounts yet. Run: claude-switch add <name>")
		return
	}
	_, _ = fmt.Fprintf(w, "%s  %-12s %-34s %-37s %-37s %s%s\n", c.dim, "ACCOUNT", "EMAIL", "SESSION (5h)", "WEEKLY (7d)", "UPDATED", c.reset)
	for _, a := range v.Accounts {
		mark := "  "
		if a.Active {
			mark = c.green + "●" + c.reset + " "
		}
		var five, fiveR, seven, sevenR *float64
		at := ""
		if u := a.Usage; u != nil {
			five, fiveR, seven, sevenR = u.Five, u.FiveReset, u.Seven, u.SevenReset
			if u.At > 0 {
				at = humanAgo(now, unix(u.At))
			}
		}
		_, _ = fmt.Fprintf(w, "%s%s%-12s%s %-34s %s   %s", mark, c.bold, a.Name, c.reset, a.Email,
			meter(c, five, fiveR, v.Threshold, now), meter(c, seven, sevenR, v.Threshold, now))
		if at != "" {
			_, _ = fmt.Fprintf(w, "   %s%s%s", c.dim, at, c.reset)
		}
		if a.Dead {
			_, _ = fmt.Fprintf(w, "  %sLOGIN NEEDED: claude-switch login %s%s", c.red, a.Name, c.reset)
		}
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintf(w, "\n%sswitches automatically at %d%% of either window%s\n", c.dim, v.Threshold, c.reset)
}

// meter draws a 10-cell bar; a window whose reset has passed shows as no data.
func meter(c palette, pct, reset *float64, threshold int, now time.Time) string {
	if pct == nil || reset == nil || unix(*reset).Before(now) {
		return fmt.Sprintf("%s%-34s%s", c.dim, "no data yet", c.reset)
	}
	p := int(math.Floor(*pct))
	filled := (p + 5) / 10
	bar := strings.Repeat("█", min(max(filled, 0), 10)) + strings.Repeat("░", 10-min(max(filled, 0), 10))
	col := c.green
	switch {
	case p >= threshold:
		col = c.red
	case p >= 75:
		col = c.yellow
	}
	return fmt.Sprintf("%s%s %3d%%%s  %sresets in %-7s%s", col, bar, p, c.reset, c.dim, humanIn(now, unix(*reset)), c.reset)
}

func unix(sec float64) time.Time { return time.Unix(int64(sec), 0) }

// humanIn formats a future time as "4h 12m" or "2d 3h".
func humanIn(now, t time.Time) string {
	s := int(t.Sub(now).Seconds())
	switch {
	case s >= 86400:
		return fmt.Sprintf("%dd %dh", s/86400, s%86400/3600)
	case s >= 3600:
		return fmt.Sprintf("%dh %dm", s/3600, s%3600/60)
	default:
		return fmt.Sprintf("%dm", s/60+1)
	}
}

func humanAgo(now, t time.Time) string {
	s := int(now.Sub(t).Seconds())
	switch {
	case s < 60:
		return "just now"
	case s < 3600:
		return fmt.Sprintf("%dm ago", s/60)
	case s < 86400:
		return fmt.Sprintf("%dh ago", s/3600)
	default:
		return fmt.Sprintf("%dd ago", s/86400)
	}
}
