package otlpjson

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	traceHex = "5b8efff798038103d269b633813fc60c"
	traceB64 = "W47/95gDgQPSabYzgT/GDA=="
	spanHex  = "eee19b7ec3c1b174"
	spanB64  = "7uGbfsPBsXQ="
)

func TestNormalizeIDsConvertsHexToBase64(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"trace id", `{"traceId":"` + traceHex + `"}`, `{"traceId":"` + traceB64 + `"}`},
		{"span id", `{"spanId":"` + spanHex + `"}`, `{"spanId":"` + spanB64 + `"}`},
		{"parent span id", `{"parentSpanId":"` + spanHex + `"}`, `{"parentSpanId":"` + spanB64 + `"}`},
		{"snake case", `{"trace_id":"` + traceHex + `","span_id":"` + spanHex + `","parent_span_id":"` + spanHex + `"}`,
			`{"trace_id":"` + traceB64 + `","span_id":"` + spanB64 + `","parent_span_id":"` + spanB64 + `"}`},
		{"uppercase hex", `{"traceId":"` + strings.ToUpper(traceHex) + `"}`, `{"traceId":"` + traceB64 + `"}`},
		{"whitespace around the colon", "{\"spanId\" \n:\t \"" + spanHex + "\"}", "{\"spanId\" \n:\t \"" + spanB64 + "\"}"},
		{"nested in spans, links and exemplars",
			`{"a":[{"b":{"spans":[{"traceId":"` + traceHex + `","links":[{"traceId":"` + traceHex + `","spanId":"` + spanHex + `"}]}],"exemplars":[{"spanId":"` + spanHex + `"}]}}]}`,
			`{"a":[{"b":{"spans":[{"traceId":"` + traceB64 + `","links":[{"traceId":"` + traceB64 + `","spanId":"` + spanB64 + `"}]}],"exemplars":[{"spanId":"` + spanB64 + `"}]}}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(NormalizeIDs([]byte(tc.in))); got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestNormalizeIDsLeavesEverythingElseAlone(t *testing.T) {
	cases := map[string]string{
		"already base64 trace id":      `{"traceId":"` + traceB64 + `"}`,
		"already base64 span id":       `{"spanId":"` + spanB64 + `"}`,
		"unpadded base64":              `{"spanId":"7uGbfsPBsXQ"}`,
		"empty string":                 `{"traceId":"","spanId":"","parentSpanId":""}`,
		"trace id of span length":      `{"traceId":"` + spanHex + `"}`,
		"span id of trace length":      `{"spanId":"` + traceHex + `"}`,
		"too short":                    `{"spanId":"eee19b7ec3c1b1"}`,
		"too long":                     `{"spanId":"eee19b7ec3c1b17400"}`,
		"not hex":                      `{"spanId":"zzz19b7ec3c1b174"}`,
		"not a string":                 `{"spanId":16,"traceId":null,"parentSpanId":true}`,
		"other keys with hex":          `{"name":"` + traceHex + `","stringValue":"` + spanHex + `"}`,
		"attribute named like an id":   `{"key":"traceId","value":{"stringValue":"` + traceHex + `"}}`,
		"id name inside a text":        `{"body":{"stringValue":"\"traceId\":\"` + traceHex + `\""}}`,
		"escaped quote before the key": `{"a":"x\\","spanId":"` + spanB64 + `"}`,
		"no ids at all":                `{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"body":{"stringValue":"hi"}}]}]}]}`,
		"empty":                        ``,
		"not json":                     `traceId`,
		"unterminated string":          `{"spanId":"` + spanHex,
		"unterminated key":             `{"spanId`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got := NormalizeIDs([]byte(in))
			if string(got) != in {
				t.Fatalf("got %q, want the input unchanged", got)
			}
		})
	}
}

func TestNormalizeIDsDoesNotCopyWhenNothingChanges(t *testing.T) {
	in := []byte(`{"spanId":"` + spanB64 + `","other":[1,2,3]}`)
	if got := NormalizeIDs(in); &got[0] != &in[0] || len(got) != len(in) {
		t.Fatalf("a body without hex ids should come back as the same slice")
	}
}

func TestNormalizeIDsDoesNotModifyItsInput(t *testing.T) {
	in := []byte(`{"spanId":"` + spanHex + `"}`)
	snapshot := bytes.Clone(in)
	NormalizeIDs(in)
	if !bytes.Equal(in, snapshot) {
		t.Fatalf("input changed to %s", in)
	}
}

func TestNormalizeIDsKeepsTheBodyValidJSON(t *testing.T) {
	// Escapes, unicode, numbers and deep nesting around the ids must survive byte for byte.
	in := `{"s":"a\"b\\","u":"\u00e9","n":1.5e3,"d":[[[[{"traceId":"` + traceHex + `","x":"\\"}]]]]}`
	out := NormalizeIDs([]byte(in))
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, out)
	}
	want := strings.Replace(in, traceHex, traceB64, 1)
	if string(out) != want {
		t.Fatalf("got %s\nwant %s", out, want)
	}
}

func TestNormalizeIDsRunsInLinearTime(t *testing.T) {
	// 50,000 spans plus a long run of keys without ids; the walk is a single pass, so this
	// finishes in milliseconds. The check is the output: every id converted, nothing lost.
	var sb strings.Builder
	sb.WriteString(`{"spans":[`)
	const n = 50000
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"traceId":"%s","spanId":"%016x","parentSpanId":"","name":"s%d"}`, traceHex, i+1, i)
	}
	sb.WriteString(`]}`)
	out := NormalizeIDs([]byte(sb.String()))
	if strings.Contains(string(out), traceHex) || strings.Count(string(out), traceB64) != n {
		t.Fatalf("not every trace id was converted")
	}
	want := base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 0, 0, 0, 0, 1})
	if !strings.Contains(string(out), `"spanId":"`+want+`"`) {
		t.Fatalf("first span id not converted to %s", want)
	}
}

func BenchmarkNormalizeIDs(b *testing.B) {
	body := []byte(strings.Repeat(`{"traceId":"`+traceHex+`","spanId":"`+spanHex+`","name":"GET /checkout","attributes":[{"key":"k","value":{"stringValue":"v"}}]},`, 5000))
	b.SetBytes(int64(len(body)))
	for i := 0; i < b.N; i++ {
		NormalizeIDs(body)
	}
}
