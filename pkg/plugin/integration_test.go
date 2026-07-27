package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// These tests talk to a real MongoDB instance. They are skipped unless
// MONGODB_URI points at one, so that `go test ./...` stays runnable without
// any infrastructure.
//
//	docker compose up -d mongodb
//	MONGODB_URI=mongodb://root:example@localhost:27017 go test ./pkg/...

const (
	testDatabase   = "grafana_plugin_integration"
	testCollection = "events"
)

// baseTime is the reference point for the seeded documents. Timestamps are
// stored as UNIX milliseconds, which is what the datasource expects.
var baseTime = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

func mongoURI(t *testing.T) string {
	t.Helper()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI is not set, skipping the MongoDB integration tests")
	}
	return uri
}

// newTestDatasource builds a datasource against the MongoDB instance under
// test and seeds it with a known set of documents.
func newTestDatasource(t *testing.T) *Datasource {
	t.Helper()

	uri := mongoURI(t)
	ctx := t.Context()

	jsonData, err := json.Marshal(JSONDataStruct{URI: uri})
	if err != nil {
		t.Fatalf("failed to marshal the datasource settings: %v", err)
	}

	instance, err := NewDatasource(ctx, backend.DataSourceInstanceSettings{JSONData: jsonData})
	if err != nil {
		t.Fatalf("NewDatasource returned an error: %v", err)
	}

	ds, ok := instance.(*Datasource)
	if !ok {
		t.Fatalf("NewDatasource returned %T, want *Datasource", instance)
	}
	t.Cleanup(ds.Dispose)

	seed(t, ds.client)

	return ds
}

// seed replaces the content of the test collection with a deterministic set of
// documents, so that the assertions below do not depend on execution order.
func seed(t *testing.T, client *mongo.Client) {
	t.Helper()

	ctx := t.Context()
	collection := client.Database(testDatabase).Collection(testCollection)

	if err := collection.Drop(ctx); err != nil {
		t.Fatalf("failed to drop the test collection: %v", err)
	}

	documents := []any{
		bson.M{"name": "alpha", "level": "info", "value": 1, "timestamp": baseTime.UnixMilli()},
		bson.M{"name": "beta", "level": "error", "value": 2, "timestamp": baseTime.Add(time.Hour).UnixMilli()},
		bson.M{"name": "gamma", "level": "info", "value": 3, "timestamp": baseTime.Add(48 * time.Hour).UnixMilli()},
	}

	if _, err := collection.InsertMany(ctx, documents); err != nil {
		t.Fatalf("failed to seed the test collection: %v", err)
	}

	t.Cleanup(func() {
		// Best effort cleanup, the next run drops the collection anyway.
		_ = collection.Drop(context.Background())
	})
}

// runQuery executes a single query and fails the test if the datasource
// returned an error response.
func runQuery(t *testing.T, ds *Datasource, model queryModel, timeRange backend.TimeRange) *data.Frame {
	t.Helper()

	rawQuery, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("failed to marshal the query: %v", err)
	}

	resp, err := ds.QueryData(t.Context(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A", JSON: rawQuery, TimeRange: timeRange}},
	})
	if err != nil {
		t.Fatalf("QueryData returned an error: %v", err)
	}

	res, ok := resp.Responses["A"]
	if !ok {
		t.Fatalf("no response for refId A, got %v", resp.Responses)
	}
	if res.Error != nil {
		t.Fatalf("query returned an error response: %v", res.Error)
	}
	if len(res.Frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(res.Frames))
	}

	return res.Frames[0]
}

// fullRange is wide enough to cover every seeded document.
func fullRange() backend.TimeRange {
	return backend.TimeRange{From: baseTime.Add(-time.Hour), To: baseTime.Add(72 * time.Hour)}
}

// fieldByName returns the frame field with the given name.
func fieldByName(t *testing.T, frame *data.Frame, name string) *data.Field {
	t.Helper()

	for _, field := range frame.Fields {
		if field.Name == name {
			return field
		}
	}

	t.Fatalf("frame has no %q field, got %v", name, frame.Fields)
	return nil
}

// stringValues collects a text field into a slice, for easier assertions.
func stringValues(t *testing.T, field *data.Field) []string {
	t.Helper()

	values := make([]string, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		value, ok := field.At(i).(string)
		if !ok {
			t.Fatalf("field %q value %d is a %T, want a string", field.Name, i, field.At(i))
		}
		values = append(values, value)
	}
	return values
}

func TestIntegrationCheckHealth(t *testing.T) {
	ds := newTestDatasource(t)

	res, err := ds.CheckHealth(t.Context(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatalf("CheckHealth returned an error: %v", err)
	}
	if res.Status != backend.HealthStatusOk {
		t.Fatalf("health status = %v (%s), want %v", res.Status, res.Message, backend.HealthStatusOk)
	}
}

func TestIntegrationCheckHealthUnreachable(t *testing.T) {
	mongoURI(t) // keep this test aligned with the rest of the integration suite

	// Port 1 is reserved and nothing listens on it, so the ping has to fail.
	client, err := mongo.Connect(mongoOptions.Client().
		ApplyURI("mongodb://127.0.0.1:1").
		SetServerSelectionTimeout(2 * time.Second))
	if err != nil {
		t.Fatalf("failed to create the MongoDB client: %v", err)
	}
	ds := &Datasource{URI: "mongodb://127.0.0.1:1", client: client}
	t.Cleanup(ds.Dispose)

	res, err := ds.CheckHealth(t.Context(), &backend.CheckHealthRequest{})
	if err != nil {
		t.Fatalf("CheckHealth returned an error: %v", err)
	}
	if res.Status != backend.HealthStatusError {
		t.Fatalf("health status = %v (%s), want %v", res.Status, res.Message, backend.HealthStatusError)
	}
}

func TestIntegrationQueryAllDocuments(t *testing.T) {
	ds := newTestDatasource(t)

	frame := runQuery(t, ds, queryModel{
		Database:   testDatabase,
		Collection: testCollection,
		QueryText:  "{}",
	}, fullRange())

	if got := frame.Rows(); got != 3 {
		t.Fatalf("got %d rows, want 3", got)
	}

	// Fields are sorted alphabetically and every document shares the same shape.
	wantFields := []string{"_id", "level", "name", "timestamp", "value"}
	gotFields := make([]string, 0, len(frame.Fields))
	for _, field := range frame.Fields {
		gotFields = append(gotFields, field.Name)
	}
	if fmt.Sprint(gotFields) != fmt.Sprint(wantFields) {
		t.Errorf("fields = %v, want %v", gotFields, wantFields)
	}

	names := stringValues(t, fieldByName(t, frame, "name"))
	if fmt.Sprint(names) != fmt.Sprint([]string{"alpha", "beta", "gamma"}) {
		t.Errorf("names = %v, want [alpha beta gamma]", names)
	}

	// _id is a BSON ObjectID and has to be rendered as its hex representation.
	for i, id := range stringValues(t, fieldByName(t, frame, "_id")) {
		if _, err := bson.ObjectIDFromHex(id); err != nil {
			t.Errorf("_id %d = %q, which is not a valid ObjectID: %v", i, id, err)
		}
	}
}

func TestIntegrationQueryWithFilter(t *testing.T) {
	ds := newTestDatasource(t)

	frame := runQuery(t, ds, queryModel{
		Database:   testDatabase,
		Collection: testCollection,
		QueryText:  `{"level": "info"}`,
	}, fullRange())

	names := stringValues(t, fieldByName(t, frame, "name"))
	if fmt.Sprint(names) != fmt.Sprint([]string{"alpha", "gamma"}) {
		t.Errorf("names = %v, want [alpha gamma]", names)
	}
}

func TestIntegrationQueryWithOperatorAndComments(t *testing.T) {
	ds := newTestDatasource(t)

	frame := runQuery(t, ds, queryModel{
		Database:   testDatabase,
		Collection: testCollection,
		QueryText: `// only keep the documents above 1
			{"value": {"$gt": 1}} /* inline comment */`,
	}, fullRange())

	names := stringValues(t, fieldByName(t, frame, "name"))
	if fmt.Sprint(names) != fmt.Sprint([]string{"beta", "gamma"}) {
		t.Errorf("names = %v, want [beta gamma]", names)
	}
}

func TestIntegrationQueryHonoursTimeRange(t *testing.T) {
	ds := newTestDatasource(t)

	// This range covers the first two documents but not the one two days later.
	frame := runQuery(t, ds, queryModel{
		Database:       testDatabase,
		Collection:     testCollection,
		QueryText:      "{}",
		TimestampField: "timestamp",
	}, backend.TimeRange{From: baseTime.Add(-time.Hour), To: baseTime.Add(2 * time.Hour)})

	names := stringValues(t, fieldByName(t, frame, "name"))
	if fmt.Sprint(names) != fmt.Sprint([]string{"alpha", "beta"}) {
		t.Errorf("names = %v, want [alpha beta]", names)
	}

	// The timestamp field has to be a real time field so that Grafana can plot it.
	timestamps := fieldByName(t, frame, "timestamp")
	if timestamps.Type() != data.FieldTypeTime {
		t.Fatalf("timestamp field type = %v, want %v", timestamps.Type(), data.FieldTypeTime)
	}
	first, ok := timestamps.At(0).(time.Time)
	if !ok {
		t.Fatalf("timestamp value is a %T, want a time.Time", timestamps.At(0))
	}
	if !first.Equal(baseTime) {
		t.Errorf("first timestamp = %v, want %v", first, baseTime)
	}
}

func TestIntegrationQueryUnknownCollection(t *testing.T) {
	ds := newTestDatasource(t)

	// Querying a collection that does not exist is not an error for MongoDB,
	// it simply returns nothing.
	frame := runQuery(t, ds, queryModel{
		Database:   testDatabase,
		Collection: "does_not_exist",
		QueryText:  "{}",
	}, fullRange())

	if got := frame.Rows(); got != 0 {
		t.Errorf("got %d rows, want 0", got)
	}
}

func TestIntegrationQueryInvalidQueryText(t *testing.T) {
	ds := newTestDatasource(t)

	rawQuery, err := json.Marshal(queryModel{
		Database:   testDatabase,
		Collection: testCollection,
		QueryText:  `{"unbalanced":`,
	})
	if err != nil {
		t.Fatalf("failed to marshal the query: %v", err)
	}

	resp, err := ds.QueryData(t.Context(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A", JSON: rawQuery, TimeRange: fullRange()}},
	})
	if err != nil {
		t.Fatalf("QueryData returned an error: %v", err)
	}

	res := resp.Responses["A"]
	if res.Error == nil {
		t.Fatal("expected an error response for a malformed query")
	}
	if res.Status != backend.StatusBadRequest {
		t.Errorf("status = %v, want %v", res.Status, backend.StatusBadRequest)
	}
}
