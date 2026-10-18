package publicpage

import (
	"fmt"
	"html/template"
	"strings"
)

// A 5x7 dot-matrix font: the hero of every public page spells a short phrase in lit dots.
var glyphs = map[rune][7]string{
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G': {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".###."},
	'H': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'I': {".###.", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'J': {"..###", "...#.", "...#.", "...#.", "...#.", "#..#.", ".##.."},
	'K': {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'O': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'S': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V': {"#...#", "#...#", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'X': {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'Z': {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2': {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#####"},
	'3': {"####.", "....#", "....#", ".###.", "....#", "....#", "####."},
	'4': {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#."},
	'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6': {".###.", "#....", "#....", "####.", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "....#", ".###."},
	'%': {"##..#", "##..#", "...#.", "..#..", ".#...", "#..##", "#..##"},
	'.': {".....", ".....", ".....", ".....", ".....", ".##..", ".##.."},
	'-': {".....", ".....", ".....", "#####", ".....", ".....", "....."},
	':': {".....", ".##..", ".##..", ".....", ".##..", ".##..", "....."},
	'/': {"....#", "....#", "...#.", "..#..", ".#...", "#....", "#...."},
	' ': {".....", ".....", ".....", ".....", ".....", ".....", "....."},
}

const (
	tickerCols = 91
	tickerRows = 7
	// MaxTickerChars is how many characters fit across the hero.
	MaxTickerChars = 14
)

// Tone picks the colour of the lit dots.
type Tone string

const (
	ToneGood  Tone = "good"
	ToneWarn  Tone = "warn"
	ToneBad   Tone = "bad"
	ToneMaint Tone = "maint"
)

var toneColors = map[Tone]string{ToneGood: "#7dffb0", ToneWarn: "#ffd29b", ToneBad: "#ff9aa8", ToneMaint: "#cdb8ff"}

// Ticker renders text as lit dots on a dim dot grid. Text longer than MaxTickerChars is cut.
func Ticker(text string, tone Tone) template.HTML {
	text = strings.ToUpper(strings.TrimSpace(text))
	runes := []rune(text)
	if len(runes) > MaxTickerChars {
		runes = runes[:MaxTickerChars]
	}
	on := toneColors[tone]
	if on == "" {
		on = toneColors[ToneGood]
	}
	off := "rgba(255,255,255,.09)"
	lit := map[[2]int]bool{}
	for i, r := range runes {
		glyph, ok := glyphs[r]
		if !ok {
			glyph = glyphs[' ']
		}
		for row, line := range glyph {
			for col, ch := range line {
				if ch == '#' {
					lit[[2]int{i*6 + col, row}] = true
				}
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="tkr" viewBox="0 0 %d %d" width="%d" height="%d" preserveAspectRatio="xMinYMid meet" role="img" aria-label="%s" xmlns="http://www.w3.org/2000/svg">`, tickerCols*10, tickerRows*10, tickerCols*10, tickerRows*10, template.HTMLEscapeString(string(runes)))
	// The dim grid is one tiled pattern; only lit dots are drawn one by one, so the markup stays small.
	fmt.Fprintf(&b, `<defs><pattern id="dots" width="10" height="10" patternUnits="userSpaceOnUse"><circle cx="5" cy="5" r="3.3" fill="%s"/></pattern></defs><rect width="%d" height="%d" fill="url(#dots)"/>`, off, tickerCols*10, tickerRows*10)
	fmt.Fprintf(&b, `<g fill="%s">`, on)
	for row := 0; row < tickerRows; row++ {
		for col := 0; col < tickerCols; col++ {
			if lit[[2]int{col, row}] {
				fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="3.3"/>`, 5+col*10, 5+row*10)
			}
		}
	}
	b.WriteString(`</g></svg>`)
	return template.HTML(b.String())
}
