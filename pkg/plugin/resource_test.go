package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// resourceCall runs a resource request and returns its status and body.
//
// Grafana sends the path and the query string separately, which is what the
// adapter expects, so `target` is split the same way here.
func resourceCall(t *testing.T, ds *Datasource, target string) (int, []byte) {
	t.Helper()

	path, _, _ := strings.Cut(target, "?")

	var status int
	var body []byte

	sender := backend.CallResourceResponseSenderFunc(func(res *backend.CallResourceResponse) error {
		status = res.Status
		body = res.Body
		return nil
	})

	err := ds.CallResource(t.Context(), &backend.CallResourceRequest{
		Method: "GET",
		Path:   path,
		URL:    target,
	}, sender)
	if err != nil {
		t.Fatalf("CallResource returned an error: %v", err)
	}

	return status, body
}

// resourceNames runs a resource request expected to return a list of names.
func resourceNames(t *testing.T, ds *Datasource, path string) []string {
	t.Helper()

	status, body := resourceCall(t, ds, path)
	if status != 200 {
		t.Fatalf("status = %d, want 200, body: %s", status, body)
	}

	var names []string
	if err := json.Unmarshal(body, &names); err != nil {
		t.Fatalf("failed to decode %s: %v", body, err)
	}
	return names
}

// resourceError runs a resource request expected to fail, and returns the message.
func resourceError(t *testing.T, ds *Datasource, path string, wantStatus int) string {
	t.Helper()

	status, body := resourceCall(t, ds, path)
	if status != wantStatus {
		t.Fatalf("status = %d, want %d, body: %s", status, wantStatus, body)
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode %s: %v", body, err)
	}
	return payload.Error
}

// The handler has to be usable on a datasource that never connected, which is
// what happens when the settings are wrong.
func TestCallResourceWithoutClient(t *testing.T) {
	ds := &Datasource{}

	for _, path := range []string{
		"databases",
		"collections?database=grafana",
		"fields?database=grafana&collection=logs",
	} {
		t.Run(path, func(t *testing.T) {
			if msg := resourceError(t, ds, path, 500); msg == "" {
				t.Error("expected a message explaining that the client is not initialized")
			}
		})
	}
}

func TestCallResourceUnknownPath(t *testing.T) {
	ds := &Datasource{}

	status, _ := resourceCall(t, ds, "nope")
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestCallResourceMissingParameters(t *testing.T) {
	ds := newTestDatasource(t)

	t.Run("collections without a database", func(t *testing.T) {
		if msg := resourceError(t, ds, "collections", 400); msg == "" {
			t.Error("expected a message about the missing database parameter")
		}
	})

	t.Run("fields without a collection", func(t *testing.T) {
		if msg := resourceError(t, ds, "fields?database="+testDatabase, 400); msg == "" {
			t.Error("expected a message about the missing collection parameter")
		}
	})
}

func TestIntegrationListDatabases(t *testing.T) {
	ds := newTestDatasource(t)

	names := resourceNames(t, ds, "databases")

	if !containsString(names, testDatabase) {
		t.Errorf("databases = %v, want it to contain %q", names, testDatabase)
	}
	if !sortedStrings(names) {
		t.Errorf("databases = %v, want them sorted", names)
	}
}

func TestIntegrationListCollections(t *testing.T) {
	ds := newTestDatasource(t)

	names := resourceNames(t, ds, "collections?database="+testDatabase)

	if !containsString(names, testCollection) {
		t.Errorf("collections = %v, want it to contain %q", names, testCollection)
	}
}

func TestIntegrationListCollectionsOfAnUnknownDatabase(t *testing.T) {
	ds := newTestDatasource(t)

	// An unknown database is not an error for MongoDB, it simply holds nothing.
	if names := resourceNames(t, ds, "collections?database=does_not_exist"); len(names) != 0 {
		t.Errorf("collections = %v, want none", names)
	}
}

func TestIntegrationListFields(t *testing.T) {
	ds := newTestDatasource(t)

	names := resourceNames(t, ds, fmt.Sprintf("fields?database=%s&collection=%s", testDatabase, testCollection))

	want := []string{"_id", "level", "name", "timestamp", "value"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Errorf("fields = %v, want %v", names, want)
	}
}

// Documents do not have to share the same shape, so the union of the sampled
// documents is reported.
func TestIntegrationListFieldsUnionOfDocuments(t *testing.T) {
	ds := newTestDatasource(t)

	collection := ds.client.Database(testDatabase).Collection("mixed")
	if err := collection.Drop(t.Context()); err != nil {
		t.Fatalf("failed to drop the collection: %v", err)
	}
	t.Cleanup(func() { _ = collection.Drop(context.Background()) })

	if _, err := collection.InsertMany(t.Context(), []any{
		map[string]any{"a": 1},
		map[string]any{"b": 2},
	}); err != nil {
		t.Fatalf("failed to seed the collection: %v", err)
	}

	names := resourceNames(t, ds, fmt.Sprintf("fields?database=%s&collection=mixed", testDatabase))
	if want := "[_id a b]"; fmt.Sprint(names) != want {
		t.Errorf("fields = %v, want %v", names, want)
	}
}

func TestIntegrationListFieldsOfAnEmptyCollection(t *testing.T) {
	ds := newTestDatasource(t)

	names := resourceNames(t, ds, fmt.Sprintf("fields?database=%s&collection=does_not_exist", testDatabase))
	if len(names) != 0 {
		t.Errorf("fields = %v, want none", names)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedStrings(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}
