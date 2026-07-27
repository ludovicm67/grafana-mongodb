package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. This datasource instance implements backend.QueryDataHandler and
// backend.CheckHealthHandler, plus instancemgmt.InstanceDisposer so that the
// MongoDB client is released when the datasource settings change.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

const (
	// How long the driver looks for a reachable server before giving up. The
	// driver default is 30 seconds, which is far too long for a health check
	// triggered from the datasource configuration page.
	defaultServerSelectionTimeout = 10 * time.Second
	// How long a single connection attempt may take.
	defaultConnectTimeout = 10 * time.Second
	// How long Dispose waits for the client to shut down cleanly.
	disconnectTimeout = 10 * time.Second
)

// JSONDataStruct holds the non-secret datasource settings.
type JSONDataStruct struct {
	Username string `json:"username"`
	URI      string `json:"uri"`
}

// Datasource is the MongoDB datasource instance. One instance is created per
// configured datasource and reused across queries, which lets the MongoDB
// driver pool connections instead of dialing on every single query.
type Datasource struct {
	URI    string
	client *mongo.Client
}

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	// Variable to hold the unmarshaled data
	var jsonData JSONDataStruct

	// Unmarshal the JSON data into the struct
	if err := json.Unmarshal(settings.JSONData, &jsonData); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON data: %w", err)
	}

	// Those are the configured fields from the datasource options
	uri := jsonData.URI
	username := jsonData.Username
	password := settings.DecryptedSecureJSONData["password"]

	datasourceURI := generateMongoURI(uri, username, password)

	// mongo.Connect does not reach out to the deployment, it only validates the
	// options and prepares the connection pool. Reachability is checked by Ping.
	//
	// The timeouts are set before ApplyURI so that they stay overridable from
	// the connection string, through `?serverSelectionTimeoutMS=` and
	// `?connectTimeoutMS=`. Without them an unreachable host would keep the
	// "Save & test" button spinning for the driver default of 30 seconds.
	clientOptions := mongoOptions.Client().
		SetServerSelectionTimeout(defaultServerSelectionTimeout).
		SetConnectTimeout(defaultConnectTimeout).
		ApplyURI(datasourceURI)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to create the MongoDB client: %w", err)
	}

	return &Datasource{
		URI:    datasourceURI,
		client: client,
	}, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewDatasource factory function.
func (d *Datasource) Dispose() {
	if d.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()

	if err := d.client.Disconnect(ctx); err != nil {
		log.DefaultLogger.Warn("failed to disconnect from MongoDB", "error", err)
	}
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	QueryText      string `json:"queryText"`
	Database       string `json:"database"`
	Collection     string `json:"collection"`
	TimestampField string `json:"timestampField"`
}

func (d *Datasource) query(ctx context.Context, _ backend.PluginContext, query backend.DataQuery) (response backend.DataResponse) {
	// Recover from panic, and turn it into an error response instead of taking
	// the whole plugin process down.
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Error("panic while running query", "refId", query.RefID, "recover", r)
			response = backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("panic while running query: %v", r))
		}
	}()

	// Unmarshal the JSON into our queryModel.
	var qm queryModel
	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	if qm.Database == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no database was specified")
	}
	if qm.Collection == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no collection was specified")
	}
	if d.client == nil {
		return backend.ErrDataResponse(backend.StatusInternal, "the MongoDB client is not initialized")
	}

	timestampField := qm.TimestampField
	hasTimestampField := timestampField != ""
	from := float64(query.TimeRange.From.UnixNano()) / float64(time.Millisecond)
	to := float64(query.TimeRange.To.UnixNano()) / float64(time.Millisecond)

	// Remove comments from the query
	queryText := strings.TrimSpace(removeComments(qm.QueryText))
	if queryText == "" {
		queryText = "{}"
	}

	// Unmarshal the query text into bson.M
	var bsonQuery bson.M
	if err := bson.UnmarshalExtJSON([]byte(queryText), false, &bsonQuery); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query unmarshal: %v", err.Error()))
	}

	collection := d.client.Database(qm.Database).Collection(qm.Collection)

	// Execute the query
	cursor, err := collection.Find(ctx, bsonQuery)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB find error: %v", err.Error()))
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.DefaultLogger.Warn("failed to close the MongoDB cursor", "error", err)
		}
	}()

	// Initialize slice to hold all documents
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("cursor all error: %v", err.Error()))
	}

	// Identify all unique fields
	fieldSet := make(map[string]struct{})
	for _, doc := range docs {
		for key := range doc {
			fieldSet[key] = struct{}{}
		}
	}

	// Collect field names in a slice, sorted alphanumerically so that the
	// resulting frame is stable between queries.
	fieldNames := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		fieldNames = append(fieldNames, key)
	}
	sort.Strings(fieldNames)

	// Filter documents on the dashboard time range when a timestamp field is set.
	filteredDocs := make([]bson.M, 0, len(docs))
	timestamps := make([]time.Time, 0, len(docs))
	for _, doc := range docs {
		if hasTimestampField {
			timestamp, ok := toEpochMillis(doc[timestampField])
			if !ok {
				continue // Skip this document
			}

			if timestamp < from || timestamp > to {
				continue // Skip this document
			}

			timestamps = append(timestamps, time.UnixMilli(int64(timestamp)).UTC())
		}

		filteredDocs = append(filteredDocs, doc)
	}

	// Create a frame to store the results
	frame := data.NewFrame("response")

	// Add sorted fields to the frame. The timestamp field is exposed as a real
	// time field so that Grafana can use it on the time axis.
	for _, key := range fieldNames {
		if hasTimestampField && key == timestampField {
			frame.Fields = append(frame.Fields, data.NewField(key, nil, timestamps))
			continue
		}

		values := make([]string, 0, len(filteredDocs))
		for _, doc := range filteredDocs {
			values = append(values, stringify(doc[key]))
		}
		frame.Fields = append(frame.Fields, data.NewField(key, nil, values))
	}

	// Add the frame to the response
	response.Frames = append(response.Frames, frame)

	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.client == nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "The MongoDB client is not initialized",
		}, nil
	}

	// Ping the database
	if err := d.client.Ping(ctx, nil); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("MongoDB ping failed: %v", err),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "MongoDB connection successful",
	}, nil
}

// stringify converts a BSON value into its string representation.
func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bson.ObjectID:
		return v.Hex()
	case bson.DateTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// toEpochMillis converts a BSON value into a UNIX timestamp in milliseconds.
// It reports whether the conversion succeeded.
func toEpochMillis(value any) (float64, bool) {
	switch v := value.(type) {
	case bson.DateTime:
		return float64(v), true
	case time.Time:
		return float64(v.UnixMilli()), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// generateMongoURI generates a MongoDB URI from the provided parameters
func generateMongoURI(uri string, username string, password string) string {
	// Check if URI already starts with "mongodb://" or "mongodb+srv://"
	if strings.HasPrefix(uri, "mongodb://") || strings.HasPrefix(uri, "mongodb+srv://") {
		if username != "" && password != "" {
			// Split the URI into two parts: protocol and the rest
			parts := strings.SplitN(uri, "://", 2)
			// Rebuild the URI with the credentials
			uri = fmt.Sprintf("%s://%s:%s@%s", parts[0], username, password, parts[1])
		}
	} else {
		if username != "" && password != "" {
			uri = fmt.Sprintf("mongodb://%s:%s@%s", username, password, uri)
		} else {
			uri = fmt.Sprintf("mongodb://%s", uri)
		}
	}

	return uri
}

// removeComments removes comments from a MongoDB query
func removeComments(query string) string {
	// Remove single-line comments
	reSingleLine := regexp.MustCompile(`//.*`)
	query = reSingleLine.ReplaceAllString(query, "")

	// Remove block comments
	reBlock := regexp.MustCompile(`/\*.*?\*/`)
	query = reBlock.ReplaceAllString(query, "")

	return query
}
