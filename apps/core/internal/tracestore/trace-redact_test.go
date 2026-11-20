package tracestore

import "testing"

// countingRedactor counts how many maps it was asked to redact and masks the key "secret".
type countingRedactor struct{ maps int }

func (c *countingRedactor) Redact(sample any) any {
	m, ok := sample.(map[string]any)
	if !ok {
		return sample
	}
	c.maps++
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k == "secret" {
			v = "[redacted]"
		}
		out[k] = v
	}
	return out
}

func TestRedactCopiesASharedResourceOnceNotOncePerSpan(t *testing.T) {
	shared := map[string]any{"service.name": "api", "secret": "s3"}
	spans := make([]Span, 500)
	for i := range spans {
		spans[i] = Span{Resource: shared}
	}
	other := map[string]any{"service.name": "worker", "secret": "w3"}
	spans = append(spans, Span{Resource: other}, Span{Resource: other})

	counter := &countingRedactor{}
	Redact(spans, counter)

	if counter.maps != 2 {
		t.Fatalf("redacted %d resource maps for 2 distinct resources", counter.maps)
	}
	for i, sp := range spans {
		if sp.Resource["secret"] != "[redacted]" {
			t.Fatalf("span %d resource not redacted: %+v", i, sp.Resource)
		}
	}
	if spans[0].Resource["service.name"] != "api" || spans[500].Resource["service.name"] != "worker" {
		t.Fatalf("resources got mixed up: %+v %+v", spans[0].Resource, spans[500].Resource)
	}
	if shared["secret"] != "s3" {
		t.Fatal("the redactor must not change the input map in place")
	}
}
