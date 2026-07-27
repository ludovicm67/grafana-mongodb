package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNewDatasource(t *testing.T) {
	instance, err := NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                json.RawMessage(`{"uri": "mongodb://localhost:27017", "username": "admin"}`),
		DecryptedSecureJSONData: map[string]string{"password": "secret"},
	})
	if err != nil {
		t.Fatalf("NewDatasource returned an error: %v", err)
	}

	ds, ok := instance.(*Datasource)
	if !ok {
		t.Fatalf("NewDatasource returned %T, want *Datasource", instance)
	}
	defer ds.Dispose()

	if want := "mongodb://admin:secret@localhost:27017"; ds.URI != want {
		t.Errorf("URI = %q, want %q", ds.URI, want)
	}
	if ds.client == nil {
		t.Error("client is nil, want an initialized MongoDB client")
	}
}

func TestNewDatasourceInvalidSettings(t *testing.T) {
	if _, err := NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData: json.RawMessage(`not json`),
	}); err == nil {
		t.Error("NewDatasource did not return an error for malformed JSON settings")
	}
}

func TestQueryDataRejectsIncompleteQueries(t *testing.T) {
	ds := &Datasource{client: nil}

	tests := []struct {
		name   string
		query  string
		status backend.Status
	}{
		{"malformed json", `not json`, backend.StatusBadRequest},
		{"no database", `{"collection": "logs", "queryText": "{}"}`, backend.StatusBadRequest},
		{"no collection", `{"database": "test", "queryText": "{}"}`, backend.StatusBadRequest},
		{"no client", `{"database": "test", "collection": "logs", "queryText": "{}"}`, backend.StatusInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(tt.query)}},
			})
			if err != nil {
				t.Fatalf("QueryData returned an error: %v", err)
			}

			if len(resp.Responses) != 1 {
				t.Fatalf("got %d responses, want 1", len(resp.Responses))
			}

			res := resp.Responses["A"]
			if res.Error == nil {
				t.Fatalf("expected an error response, got frames: %v", res.Frames)
			}
			if res.Status != tt.status {
				t.Errorf("status = %v, want %v", res.Status, tt.status)
			}
		})
	}
}

func TestCheckHealthWithoutClient(t *testing.T) {
	ds := &Datasource{}

	res, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatalf("CheckHealth returned an error: %v", err)
	}
	if res.Status != backend.HealthStatusError {
		t.Errorf("status = %v, want %v", res.Status, backend.HealthStatusError)
	}
}

func TestDisposeWithoutClient(t *testing.T) {
	// Dispose is called by the SDK even for instances that failed to fully
	// initialize, so it has to stay safe on a zero value.
	(&Datasource{}).Dispose()
}

func TestGenerateMongoURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		username string
		password string
		want     string
	}{
		{"host only", "localhost:27017", "", "", "mongodb://localhost:27017"},
		{"host with credentials", "localhost:27017", "admin", "pass", "mongodb://admin:pass@localhost:27017"},
		{"full uri", "mongodb://localhost:27017", "", "", "mongodb://localhost:27017"},
		{"full uri with credentials", "mongodb://localhost:27017", "admin", "pass", "mongodb://admin:pass@localhost:27017"},
		{"srv uri", "mongodb+srv://cluster.example.com", "admin", "pass", "mongodb+srv://admin:pass@cluster.example.com"},
		{"username without password is ignored", "localhost:27017", "admin", "", "mongodb://localhost:27017"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateMongoURI(tt.uri, tt.username, tt.password); got != tt.want {
				t.Errorf("generateMongoURI(%q, %q, %q) = %q, want %q", tt.uri, tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestRemoveComments(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"no comment", `{"a": 1}`, `{"a": 1}`},
		{"single line comment", "{\"a\": 1} // keep only the a field", `{"a": 1} `},
		{"block comment", `{"a": /* inline */ 1}`, `{"a":  1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeComments(tt.query); got != tt.want {
				t.Errorf("removeComments(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
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

// TestQueryTextIsExtendedJSON documents that queries are parsed as MongoDB
// extended JSON, which is a superset of JSON: plain queries and operators keep
// working, and `$oid` / `$date` wrappers become real BSON values.
func TestQueryTextIsExtendedJSON(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"empty", `{}`},
		{"equality", `{"name": "test"}`},
		{"comparison operator", `{"value": {"$gt": 5}}`},
		{"logical operator", `{"$and": [{"a": 1}, {"b": 2}]}`},
		{"object id", `{"_id": {"$oid": "507f1f77bcf86cd799439011"}}`},
		{"date", `{"ts": {"$gte": {"$date": "2024-01-01T00:00:00Z"}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed bson.M
			if err := bson.UnmarshalExtJSON([]byte(tt.query), false, &parsed); err != nil {
				t.Errorf("failed to parse %s: %v", tt.query, err)
			}
		})
	}
}
