package tracestore

import (
	"encoding/json"
	"sort"
	"strings"
)

// MaxResourceBytes caps the encoded resource attributes stored on each span. The same resource is
// copied onto every span of its batch, so an unbounded map would multiply by the span count.
const MaxResourceBytes = 8 << 10

// spanRowOverhead is the fixed number of bytes Span.Size adds on top of the stored text columns. It
// stands for the app id, the integer columns and the row header, so Size is the stored column bytes
// plus this constant and never less than what the row really takes.
const spanRowOverhead = 128

// resourceInfo is one OTLP resource after limits: what is stored on each of its spans.
type resourceInfo struct {
	attrs   map[string]any
	service string
	// dropped counts resource attributes cut by the attribute and size caps.
	dropped int
	// jsonLen is the encoded length of attrs, the bytes each span stores for it.
	jsonLen int
}

// identifyingResourceKeys are kept before any other resource attribute when the cap bites, because
// they say which service, version and host a span came from. Keys under deployment.environment.,
// k8s. and cloud. follow them.
var identifyingResourceKeys = []string{
	"service.name", "service.namespace", "service.version", "service.instance.id",
	"host.name", "deployment.environment",
}

func resourceRank(key string) int {
	for i, k := range identifyingResourceKeys {
		if key == k {
			return i
		}
	}
	if strings.HasPrefix(key, "deployment.environment.") || strings.HasPrefix(key, "k8s.") || strings.HasPrefix(key, "cloud.") {
		return len(identifyingResourceKeys)
	}
	return len(identifyingResourceKeys) + 1
}

// capResource keeps at most MaxAttributesPerSpan resource attributes while their encoded size stays within limit: the identifying
// keys first, then the rest in sorted key order. An attribute that does not fit is skipped and a
// smaller later one may still fit. It returns the kept map; the caller counts the difference.
func capResource(in map[string]any, limit int) map[string]any {
	if len(in) == 0 {
		return in
	}
	if b, err := json.Marshal(in); err == nil && len(b) <= limit && len(in) <= MaxAttributesPerSpan {
		return in
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := resourceRank(keys[i]), resourceRank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	out := make(map[string]any, len(keys))
	used := 2 // the braces
	for _, k := range keys {
		kb, err1 := json.Marshal(k)
		vb, err2 := json.Marshal(in[k])
		if err1 != nil || err2 != nil {
			continue
		}
		entry := len(kb) + 1 + len(vb) + 1 // key, colon, value, comma
		if used+entry > limit || len(out) >= MaxAttributesPerSpan {
			continue
		}
		used += entry
		out[k] = in[k]
	}
	return out
}

// newResourceInfo applies the attribute and size caps to a decoded resource. rawCount is how many
// attributes the sender put on it.
func newResourceInfo(raw map[string]any, rawCount int) resourceInfo {
	attrs := capResource(raw, MaxResourceBytes)
	service, _ := attrs["service.name"].(string)
	return resourceInfo{attrs: attrs, service: service, dropped: max(rawCount-len(attrs), 0), jsonLen: len(encodeJSON(attrs))}
}

// storedSize is the bytes a span takes in its row: every stored text column plus spanRowOverhead.
// The writer recomputes it from the encoded columns so it stays true after redaction.
func storedSize(sp Span, attributes, resource, events, links int) int {
	return len(sp.TraceID) + len(sp.SpanID) + len(sp.ParentSpanID) + len(sp.Name) + len(sp.StatusMessage) + len(sp.ServiceName) +
		attributes + resource + events + links + spanRowOverhead
}
