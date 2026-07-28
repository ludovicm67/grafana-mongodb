package plugin

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestQueryTypeDefaultsToFind(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{"empty means find", "", QueryTypeFind},
		{"find", "find", QueryTypeFind},
		{"aggregate", "aggregate", QueryTypeAggregate},
		{"count", "count", QueryTypeCount},
		{"distinct", "distinct", QueryTypeDistinct},
		{"case insensitive", "Aggregate", QueryTypeAggregate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (queryModel{QueryType: tt.given}).queryType(); got != tt.want {
				t.Errorf("queryType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDocument(t *testing.T) {
	tests := []struct {
		name    string
		given   string
		want    bson.M
		wantErr bool
	}{
		{"empty matches everything", "", bson.M{}, false},
		{"only a comment matches everything", "// nothing here", bson.M{}, false},
		{"empty document", "{}", bson.M{}, false},
		{"equality", `{"name": "test"}`, bson.M{"name": "test"}, false},
		{"comment is stripped", "// keep the infos\n{\"level\": \"info\"}", bson.M{"level": "info"}, false},
		{"malformed", `{"unbalanced":`, nil, true},
		{"not a document", `["a"]`, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDocument(tt.given)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDocument(%q) did not return an error", tt.given)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDocument(%q) returned an error: %v", tt.given, err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("parseDocument(%q) = %v, want %v", tt.given, got, tt.want)
			}
		})
	}
}

// A sort is only meaningful if the order of its keys survives parsing.
func TestParseOrderedDocumentKeepsOrder(t *testing.T) {
	got, err := parseOrderedDocument(`{"level": 1, "name": -1, "value": 1}`)
	if err != nil {
		t.Fatalf("parseOrderedDocument returned an error: %v", err)
	}

	keys := make([]string, 0, len(got))
	for _, element := range got {
		keys = append(keys, element.Key)
	}

	if want := "[level name value]"; fmt.Sprint(keys) != want {
		t.Errorf("keys = %v, want %v", keys, want)
	}
}

func TestParsePipeline(t *testing.T) {
	tests := []struct {
		name      string
		given     string
		wantLen   int
		wantErr   bool
		errSubstr string
	}{
		{name: "empty", given: "", wantLen: 0},
		{name: "empty array", given: "[]", wantLen: 0},
		{name: "single stage", given: `[{"$match": {"level": "info"}}]`, wantLen: 1},
		{
			name:    "several stages",
			given:   `[{"$match": {}}, {"$group": {"_id": "$level", "total": {"$sum": 1}}}, {"$sort": {"total": -1}}]`,
			wantLen: 3,
		},
		{name: "comments are stripped", given: "// group them\n[{\"$match\": {}}]", wantLen: 1},
		{name: "a document is not a pipeline", given: `{"$match": {}}`, wantErr: true, errSubstr: "array of stages"},
		{name: "malformed", given: `[{"$match":`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePipeline(tt.given)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePipeline(%q) did not return an error", tt.given)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not mention %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePipeline(%q) returned an error: %v", tt.given, err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("parsePipeline(%q) has %d stages, want %d", tt.given, len(got), tt.wantLen)
			}
		})
	}
}

func TestWithTimeRange(t *testing.T) {
	tr := timeRange{from: 1000, to: 2000}

	t.Run("no timestamp field leaves the filter untouched", func(t *testing.T) {
		filter := bson.M{"level": "info"}
		got := withTimeRange(filter, "", tr)
		if fmt.Sprint(got) != fmt.Sprint(filter) {
			t.Errorf("filter = %v, want %v", got, filter)
		}
	})

	t.Run("an empty filter becomes the range alone", func(t *testing.T) {
		got := withTimeRange(bson.M{}, "ts", tr)
		if _, ok := got["$or"]; !ok {
			t.Fatalf("filter = %v, want an $or on the timestamp", got)
		}
		if _, ok := got["$and"]; ok {
			t.Errorf("filter = %v, want no $and for an empty filter", got)
		}
	})

	t.Run("an existing filter is combined, not replaced", func(t *testing.T) {
		got := withTimeRange(bson.M{"level": "info"}, "ts", tr)
		and, ok := got["$and"].(bson.A)
		if !ok {
			t.Fatalf("filter = %v, want an $and", got)
		}
		if len(and) != 2 {
			t.Fatalf("$and has %d members, want 2", len(and))
		}
		// The original filter has to be preserved as is.
		if fmt.Sprint(and[0]) != fmt.Sprint(bson.M{"level": "info"}) {
			t.Errorf("first member = %v, want the original filter", and[0])
		}
	})

	// A timestamp stored as a number and one stored as a date are never
	// comparable in BSON, so both representations have to be covered.
	t.Run("covers numbers and dates", func(t *testing.T) {
		got := withTimeRange(bson.M{}, "ts", tr)
		or, ok := got["$or"].(bson.A)
		if !ok || len(or) != 2 {
			t.Fatalf("filter = %v, want an $or with 2 members", got)
		}

		numeric := or[0].(bson.M)["ts"].(bson.M)
		if _, ok := numeric["$gte"].(int64); !ok {
			t.Errorf("first member uses %T for $gte, want int64", numeric["$gte"])
		}

		dated := or[1].(bson.M)["ts"].(bson.M)
		if _, ok := dated["$gte"].(bson.DateTime); !ok {
			t.Errorf("second member uses %T for $gte, want bson.DateTime", dated["$gte"])
		}
	})
}

func TestDocumentsToFrame(t *testing.T) {
	docs := []bson.M{
		{"name": "alpha", "value": 1},
		{"name": "beta", "other": true},
	}

	frame := documentsToFrame(docs, "", timeRange{})

	// Every field of every document shows up, sorted alphabetically.
	names := make([]string, 0, len(frame.Fields))
	for _, field := range frame.Fields {
		names = append(names, field.Name)
	}
	if want := "[name other value]"; fmt.Sprint(names) != want {
		t.Errorf("fields = %v, want %v", names, want)
	}

	if got := frame.Rows(); got != 2 {
		t.Errorf("got %d rows, want 2", got)
	}

	// A field missing from a document is rendered as an empty string.
	other := frame.Fields[1]
	if got := other.At(0); got != "" {
		t.Errorf("missing value = %q, want an empty string", got)
	}
}

func TestDocumentsToFrameDropsDocumentsOutsideTheRange(t *testing.T) {
	docs := []bson.M{
		{"name": "before", "ts": int64(500)},
		{"name": "inside", "ts": int64(1500)},
		{"name": "after", "ts": int64(2500)},
		{"name": "unparseable", "ts": "not a timestamp"},
	}

	frame := documentsToFrame(docs, "ts", timeRange{from: 1000, to: 2000})

	if got := frame.Rows(); got != 1 {
		t.Fatalf("got %d rows, want 1", got)
	}

	timestamps := frame.Fields[1]
	if timestamps.Name != "ts" {
		t.Fatalf("second field is %q, want ts", timestamps.Name)
	}
	if timestamps.Type() != data.FieldTypeTime {
		t.Errorf("timestamp field type = %v, want %v", timestamps.Type(), data.FieldTypeTime)
	}
}

func TestRunQueryRejectsAnUnknownType(t *testing.T) {
	ds := &Datasource{}

	resp, err := ds.QueryData(t.Context(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{
			RefID: "A",
			JSON:  []byte(`{"database": "db", "collection": "c", "queryType": "explain"}`),
		}},
	})
	if err != nil {
		t.Fatalf("QueryData returned an error: %v", err)
	}

	res := resp.Responses["A"]
	if res.Error == nil {
		t.Fatal("expected an error response for an unknown query type")
	}
	if res.Status != backend.StatusBadRequest {
		t.Errorf("status = %v, want %v", res.Status, backend.StatusBadRequest)
	}
	if !strings.Contains(res.Error.Error(), "unknown query type") {
		t.Errorf("error = %q, want it to mention the unknown query type", res.Error.Error())
	}
}

func TestStringify(t *testing.T) {
	objectID, err := bson.ObjectIDFromHex("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatalf("failed to build an ObjectID: %v", err)
	}
	timestamp := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int32", int32(42), "42"},
		{"float", 4.2, "4.2"},
		{"bool", true, "true"},
		{"object id", objectID, "507f1f77bcf86cd799439011"},
		{"bson datetime", bson.NewDateTimeFromTime(timestamp), "2024-01-01T12:00:00Z"},
		{"time", timestamp, "2024-01-01T12:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringify(tt.value); got != tt.want {
				t.Errorf("stringify(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestToEpochMillis(t *testing.T) {
	timestamp := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	millis := float64(timestamp.UnixMilli())

	tests := []struct {
		name  string
		value any
		want  float64
		ok    bool
	}{
		{"bson datetime", bson.NewDateTimeFromTime(timestamp), millis, true},
		{"time", timestamp, millis, true},
		{"int64", int64(1704067200000), millis, true},
		{"int32", int32(1000), 1000, true},
		{"float64", millis, millis, true},
		{"numeric string", "1704067200000", millis, true},
		{"non numeric string", "not a timestamp", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toEpochMillis(tt.value)
			if ok != tt.ok {
				t.Fatalf("toEpochMillis(%v) ok = %v, want %v", tt.value, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("toEpochMillis(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
