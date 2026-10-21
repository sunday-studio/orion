package cli

import (
	"regexp"
	"strings"

	"orion/agent/internal/ui"
)

// sanitizeMarkdown drops control characters other than newline and tab, so text
// from the network can never move the cursor or rewrite the screen.
func sanitizeMarkdown(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		switch {
		case r < 0x20, r == 0x7f, r >= 0x80 && r < 0xa0:
			return -1
		case r == 0x2028, r == 0x2029, r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
			return -1 // line separators and bidi overrides can make text read differently from what it is
		}
		return r
	}, s)
}

var (
	mdBold    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdCode    = regexp.MustCompile("`([^`]+)`")
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdHeading = regexp.MustCompile(`^#{1,6}\s`) // "#123 fixed" is text, not a heading
	mdOrder   = regexp.MustCompile(`^(\d+)[.)]\s+(.*)$`)
)

// inlineMarkdown styles **bold**, `code` and [text](url); anything else stays as written.
func inlineMarkdown(th *ui.Theme, s string) string {
	s = mdLink.ReplaceAllString(s, "$1 ($2)")
	s = mdBold.ReplaceAllStringFunc(s, func(m string) string { return th.Bold(mdBold.FindStringSubmatch(m)[1]) })
	return mdCode.ReplaceAllStringFunc(s, func(m string) string { return th.Accent(mdCode.FindStringSubmatch(m)[1]) })
}

// wrapText breaks s at spaces so each line is at most width cells (ignoring styling). A word
// longer than the width stays whole.
func wrapText(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && visibleWidth(line)+1+visibleWidth(w) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// renderMarkdown formats md for the terminal: headings, bullets, numbered lists, fenced code,
// quotes, **bold**, `code` and links. Without color (plain output, pipes, accessible mode) the
// markdown is printed as written, which is readable on its own. Tables and other markup that
// this small renderer does not know are left as written.
func renderMarkdown(th *ui.Theme, md string, width int) string {
	md = sanitizeMarkdown(md)
	if th.Plain() {
		return strings.TrimRight(md, "\n") + "\n"
	}
	if width < 40 || width > 100 {
		width = 80
	}
	width -= 4
	var out []string
	inCode := false
	var para []string
	flush := func() {
		for _, l := range wrapText(inlineMarkdown(th, strings.Join(para, " ")), width) {
			out = append(out, "  "+l)
		}
		para = nil
	}
	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flush()
			inCode = !inCode
		case inCode:
			out = append(out, "    "+th.Muted(line))
		case trimmed == "":
			flush()
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
		case mdHeading.MatchString(trimmed):
			flush()
			text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if strings.HasPrefix(trimmed, "# ") {
				out = append(out, "", " "+th.Accent("◆")+" "+th.Bold(text), "")
			} else {
				out = append(out, "  "+th.Bold(text))
			}
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			flush()
			for j, l := range wrapText(inlineMarkdown(th, trimmed[2:]), width-2) {
				mark := "  "
				if j == 0 {
					mark = th.Muted("•") + " "
				}
				out = append(out, "  "+mark+l)
			}
		case mdOrder.MatchString(trimmed):
			flush()
			m := mdOrder.FindStringSubmatch(trimmed)
			for j, l := range wrapText(inlineMarkdown(th, m[2]), width-3) {
				mark := "   "
				if j == 0 {
					mark = th.Muted(m[1]+".") + " "
				}
				out = append(out, "  "+mark+l)
			}
		case strings.HasPrefix(trimmed, ">"):
			flush()
			out = append(out, "  "+th.Muted("│ "+inlineMarkdown(th, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))))
		case strings.HasPrefix(trimmed, "|"):
			flush()
			out = append(out, "  "+trimmed) // tables stay as written
		default:
			para = append(para, trimmed)
		}
	}
	flush()
	return strings.Trim(strings.Join(out, "\n"), "\n") + "\n"
}
