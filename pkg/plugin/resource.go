package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongoOptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The datasource answers resource calls, which is what the query editor uses to
// fill its database, collection and field dropdowns.
var _ backend.CallResourceHandler = (*Datasource)(nil)

// fieldSampleSize is how many documents are read to work out which fields a
// collection holds. MongoDB has no schema, so the field names have to be
// discovered from the documents themselves.
const fieldSampleSize = 100

// CallResource answers the resource calls made by the query editor.
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	d.resourcesOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /databases", d.handleDatabases)
		mux.HandleFunc("GET /collections", d.handleCollections)
		mux.HandleFunc("GET /fields", d.handleFields)
		d.resources = httpadapter.New(mux)
	})

	return d.resources.CallResource(ctx, req, sender)
}

// handleDatabases lists the databases of the instance.
func (d *Datasource) handleDatabases(w http.ResponseWriter, r *http.Request) {
	if d.client == nil {
		writeError(w, http.StatusInternalServerError, "the MongoDB client is not initialized")
		return
	}

	names, err := d.client.ListDatabaseNames(r.Context(), bson.M{})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("unable to list the databases: %v", err))
		return
	}

	sort.Strings(names)
	writeJSON(w, http.StatusOK, names)
}

// handleCollections lists the collections of a database.
func (d *Datasource) handleCollections(w http.ResponseWriter, r *http.Request) {
	if d.client == nil {
		writeError(w, http.StatusInternalServerError, "the MongoDB client is not initialized")
		return
	}

	database := r.URL.Query().Get("database")
	if database == "" {
		writeError(w, http.StatusBadRequest, "the database query parameter is required")
		return
	}

	names, err := d.client.Database(database).ListCollectionNames(r.Context(), bson.M{})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("unable to list the collections: %v", err))
		return
	}

	sort.Strings(names)
	writeJSON(w, http.StatusOK, names)
}

// handleFields lists the field names found in a sample of the documents of a
// collection. Only the top level fields are reported, a nested path can still
// be typed by hand in the editor.
func (d *Datasource) handleFields(w http.ResponseWriter, r *http.Request) {
	if d.client == nil {
		writeError(w, http.StatusInternalServerError, "the MongoDB client is not initialized")
		return
	}

	database := r.URL.Query().Get("database")
	collection := r.URL.Query().Get("collection")
	if database == "" || collection == "" {
		writeError(w, http.StatusBadRequest, "the database and collection query parameters are required")
		return
	}

	cursor, err := d.client.Database(database).Collection(collection).
		Find(r.Context(), bson.M{}, mongoOptions.Find().SetLimit(fieldSampleSize))
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("unable to read the collection: %v", err))
		return
	}
	defer func() {
		if err := cursor.Close(r.Context()); err != nil {
			log.DefaultLogger.Warn("failed to close the MongoDB cursor", "error", err)
		}
	}()

	var docs []bson.M
	if err := cursor.All(r.Context(), &docs); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("unable to read the collection: %v", err))
		return
	}

	fieldSet := make(map[string]struct{})
	for _, doc := range docs {
		for key := range doc {
			fieldSet[key] = struct{}{}
		}
	}

	names := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		names = append(names, key)
	}
	sort.Strings(names)

	writeJSON(w, http.StatusOK, names)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.DefaultLogger.Error("failed to write the resource response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
