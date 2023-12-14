package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/mongo/options"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

type JSONDataStruct struct {
	Username string `json:"username"`
	URI      string `json:"uri"`
}

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	// Variable to hold the unmarshaled data
	var jsonData JSONDataStruct

	// Unmarshal the JSON data into the struct
	err := json.Unmarshal([]byte(settings.JSONData), &jsonData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON data: %w", err)
	}

	// Those are the configured fields from the datasource options
	uri := jsonData.URI
	username := jsonData.Username
	password := settings.DecryptedSecureJSONData["password"]

	datasourceURI := generateMongoURI(uri, username, password)

	return &Datasource{
		URI: datasourceURI,
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	URI string
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
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
	QueryText  string `json:"queryText"`
	Database   string `json:"database"`
	Collection string `json:"collection"`
}

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse

	// Recover from panic, and log the error
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Error(fmt.Sprintf(">>>>>>>> PANIC!!!: %v", r))
		}
	}()

	// Unmarshal the JSON into our queryModel.
	var qm queryModel
	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// Remove comments from the query
	queryText := removeComments(qm.QueryText)

	// Connect to the database
	client, err := mongo.Connect(ctx, mongoOptions.Client().ApplyURI(d.URI))
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unable to connect to MongoDB: %v", err.Error()))
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB ping failed: %v", err))
	}

	// Get the database
	database := client.Database(qm.Database)
	collection := database.Collection(qm.Collection)

	// Unmarshal the query text into bson.M
	var bsonQuery bson.M
	if err := json.Unmarshal([]byte(queryText), &bsonQuery); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query unmarshal: %v", err.Error()))
	}

	// Execute the query
	cursor, err := collection.Find(ctx, bsonQuery)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB find error: %v", err.Error()))
	}
	defer cursor.Close(ctx)

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

	// Fill missing fields with an empty string and convert all fields to strings
	for _, doc := range docs {
		for key := range fieldSet {
			if _, ok := doc[key]; !ok {
				doc[key] = ""
			} else {
				// Convert each field value to a string
				switch v := doc[key].(type) {
				case primitive.ObjectID:
					doc[key] = v.Hex() // Convert ObjectID to string
				default:
					doc[key] = fmt.Sprintf("%v", doc[key]) // Convert other types to string
				}
			}
		}
	}

	// Create a frame to store the results
	frame := data.NewFrame("response")

	// Collect field names in a slice
	fieldNames := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		fieldNames = append(fieldNames, key)
	}

	// Sort field names alphanumerically
	sort.Strings(fieldNames)

	// Add sorted fields to the frame
	for _, key := range fieldNames {
		var values []string // Use a slice of strings
		for _, doc := range docs {
			val := doc[key]
			var strVal string
			switch v := val.(type) {
			case primitive.ObjectID:
				strVal = v.Hex() // Convert ObjectID to string
			default:
				strVal = fmt.Sprintf("%v", val) // Convert other types to string
			}
			values = append(values, strVal)
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
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var status = backend.HealthStatusOk
	var message = "MongoDB connection successful"

	// Connect to the database
	client, err := mongo.Connect(ctx, mongoOptions.Client().ApplyURI(d.URI))
	if err != nil {
		status = backend.HealthStatusError
		message = fmt.Sprintf("Unable to connect to MongoDB: %s", err.Error())
		return &backend.CheckHealthResult{
			Status:  status,
			Message: message,
		}, nil
	}
	defer client.Disconnect(ctx)

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		status = backend.HealthStatusError
		message = fmt.Sprintf("MongoDB ping failed: %v", err)
		return &backend.CheckHealthResult{
			Status:  status,
			Message: message,
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
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
