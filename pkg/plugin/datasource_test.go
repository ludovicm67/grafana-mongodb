package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
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
