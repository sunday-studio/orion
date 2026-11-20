package metricstore

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// resourceLabelKeys are the resource attributes that tell instances of one app apart. The others are
// per app and would only add noise to the series identity.
var resourceLabelKeys = []string{"service.name", "service.instance.id", "host.name"}

// Series kinds an OTLP metric is stored as.
const (
	KindGauge     = "gauge"
	KindSum       = "sum"
	KindHistogram = "histogram"
	KindSummary   = "summary"
)

// seriesIdentity builds the key and the canonical labels JSON of a data point: its resource labels, then
// its attributes (the first maxAttributes by key, so a point with more attributes than the limit maps to
// one series however its exporter orders them). Gauges and sums keep the key "name|labels"; histograms and
// summaries are prefixed with their kind so a metric that changes type does not mix two storage layouts
// in one series.
func seriesIdentity(name, kind string, resource map[string]string, attributes []*commonpb.KeyValue) (key, labels string) {
	set := map[string]string{}
	for k, val := range resource {
		set["resource."+k] = val
	}
	attrs := make([]*commonpb.KeyValue, 0, len(attributes))
	for _, kv := range attributes {
		if kv.GetKey() != "" {
			attrs = append(attrs, kv)
		}
	}
	sort.SliceStable(attrs, func(i, j int) bool { return attrs[i].GetKey() < attrs[j].GetKey() })
	count := 0
	for _, kv := range attrs {
		if count == maxAttributes {
			break
		}
		if k := clip(kv.GetKey(), maxAttributeText); k != "" {
			set[k] = clip(anyText(kv.GetValue()), maxAttributeText)
			count++
		}
	}
	if len(set) > 0 {
		b, _ := json.Marshal(set) // map keys marshal sorted, so the identity is stable
		labels = string(b)
	}
	key = name + "|" + labels
	if kind == KindHistogram || kind == KindSummary {
		key = kind + ":" + key
	}
	return key, labels
}

// pointTime reads a data point's timestamp (now when it has none) and refuses one that is too old or too
// far ahead.
func pointTime(ns uint64, now time.Time) (time.Time, string) {
	at := now
	if ns != 0 {
		at = time.Unix(0, int64(ns)).UTC()
	}
	if at.Before(now.Add(-maxPointAge)) || at.After(now.Add(maxPointSkew)) {
		return time.Time{}, "timestamp is more than 24 hours old or in the future"
	}
	return at, ""
}

func startTime(ns uint64) time.Time {
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(ns)).UTC()
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func resourceLabels(attrs []*commonpb.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range attrs {
		for _, want := range resourceLabelKeys {
			if kv.GetKey() == want {
				out[want] = clip(anyText(kv.GetValue()), maxAttributeText)
			}
		}
	}
	return out
}

func anyText(v *commonpb.AnyValue) string {
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case nil:
		return ""
	}
	b, _ := protojson.Marshal(v)
	return string(b)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
