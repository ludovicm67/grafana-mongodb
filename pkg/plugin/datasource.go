package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

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

	// The resource handler is built on first use, so that a zero value
	// Datasource stays usable.
	resourcesOnce sync.Once
	resources     backend.CallResourceHandler
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

	if err := qm.validate(); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if d.client == nil {
		return backend.ErrDataResponse(backend.StatusInternal, "the MongoDB client is not initialized")
	}

	collection := d.client.Database(qm.Database).Collection(qm.Collection)

	return runQuery(ctx, collection, qm, newTimeRange(query.TimeRange))
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
