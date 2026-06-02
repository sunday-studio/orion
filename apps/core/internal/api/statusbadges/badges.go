package statusbadges

import (
	"fmt"
	"html"
	"math"
	"strings"
	"unicode/utf8"
)

const CacheControl = "public, max-age=60, stale-while-revalidate=120"

func RenderSVG(label string, message string, color string) string {
	label = truncateText(label, 48)
	message = truncateText(message, 32)

	labelWidth := textWidth(label, 72, 240)
	messageWidth := textWidth(message, 92, 220)
	totalWidth := labelWidth + messageWidth
	labelTextX := labelWidth / 2
	messageTextX := labelWidth + messageWidth/2

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s"><title>%s: %s</title><linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#fff" stop-opacity=".08"/><stop offset="1" stop-opacity=".08"/></linearGradient><clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#r)"><rect width="%d" height="20" fill="#555"/><rect x="%d" width="%d" height="20" fill="%s"/><rect width="%d" height="20" fill="url(#s)"/></g><g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11"><text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text><text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text></g></svg>`,
		totalWidth,
		html.EscapeString(label),
		html.EscapeString(message),
		html.EscapeString(label),
		html.EscapeString(message),
		totalWidth,
		labelWidth,
		labelWidth,
		messageWidth,
		color,
		totalWidth,
		labelTextX,
		html.EscapeString(label),
		labelTextX,
		html.EscapeString(label),
		messageTextX,
		html.EscapeString(message),
		messageTextX,
		html.EscapeString(message),
	)
}

func Color(status string) string {
	switch status {
	case "operational":
		return "#15803d"
	case "degraded":
		return "#ca8a04"
	case "partial_outage":
		return "#ea580c"
	case "major_outage":
		return "#dc2626"
	case "maintenance":
		return "#2563eb"
	default:
		return "#64748b"
	}
}

func textWidth(value string, minWidth int, maxWidth int) int {
	width := utf8.RuneCountInString(value)*7 + 18
	return int(math.Max(float64(minWidth), math.Min(float64(maxWidth), float64(width))))
}

func truncateText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= limit {
		return value
	}

	runes := []rune(value)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "..."
}
