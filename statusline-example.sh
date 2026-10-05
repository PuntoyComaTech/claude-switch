#!/usr/bin/env bash
# Minimal Claude Code status line with claude-switch.
# Shows: account · model · 5h usage · weekly usage
input=$(cat)

command -v claude-switch >/dev/null && { printf '%s' "$input" | claude-switch report >/dev/null 2>&1 & }

account=$(claude-switch current 2>/dev/null)
model=$(jq -r '.model.display_name // empty' <<<"$input")
five=$(jq -r '.rate_limits.five_hour.used_percentage // empty | floor' <<<"$input")
week=$(jq -r '.rate_limits.seven_day.used_percentage // empty | floor' <<<"$input")

out="${account:-?}"
[ -n "$model" ] && out="$out · $model"
[ -n "$five" ] && out="$out · 5h ${five}%"
[ -n "$week" ] && out="$out · 7d ${week}%"
printf '%s' "$out"
