package publicpage

import (
	"strings"
	"testing"
)

func TestTickerDrawsLitDotsForKnownGlyphsAndCutsLongText(t *testing.T) {
	svg := string(Ticker("ok", ToneGood))
	if got := strings.Count(svg, "<circle"); got < 10 || got > 80 {
		t.Fatalf("drew %d circles, want the grid pattern plus a few lit dots for two letters", got)
	}
	if !strings.Contains(svg, toneColors[ToneGood]) || !strings.Contains(svg, `aria-label="OK"`) {
		t.Fatalf("lit dots or label missing: %.120s", svg)
	}
	long := string(Ticker("this text is far too long to fit", ToneWarn))
	if !strings.Contains(long, `aria-label="THIS TEXT IS F"`) {
		t.Fatalf("long text was not cut to %d chars", MaxTickerChars)
	}
	if strings.Contains(string(Ticker("<script>", ToneBad)), "<script>") {
		t.Fatalf("ticker label must be escaped")
	}
}

func TestRenderEscapesPageData(t *testing.T) {
	out, err := Render("alert-email.html", AlertEmailData{Title: "t", Heading: "<b>x</b>"})
	if err != nil || strings.Contains(string(out), "<b>x</b>") {
		t.Fatalf("render err=%v, heading must be escaped", err)
	}
}
