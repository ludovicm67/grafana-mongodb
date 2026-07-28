package plugin

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The supported query types. They map to the equivalent MongoDB operations.
const (
	QueryTypeFind      = "find"
	QueryTypeAggregate = "aggregate"
	QueryTypeCount     = "count"
	QueryTypeDistinct  = "distinct"
)

// queryModel is the query as it is sent by the query editor.
type queryModel struct {
	// QueryType selects the MongoDB operation to run. An empty value means
	// "find", so that queries saved before the other types existed keep working.
	QueryType string `json:"queryType"`

	Database   string `json:"database"`
	Collection string `json:"collection"`

	// QueryText is the filter document, used by find, count and distinct.
	QueryText string `json:"queryText"`

	// Find only.
	Projection string `json:"projection"`
	Sort       string `json:"sort"`
	Limit      int64  `json:"limit"`
	Skip       int64  `json:"skip"`

	// Aggregate only: the pipeline, as an array of stages.
	Pipeline string `json:"pipeline"`

	// Distinct only: the field to collect the unique values of.
	DistinctField string `json:"distinctField"`

	// TimestampField holds the name of the field carrying the document date.
	TimestampField string `json:"timestampField"`
}

// queryType returns the operation to run, defaulting to find.
func (q queryModel) queryType() string {
	if q.QueryType == "" {
		return QueryTypeFind
	}
	return strings.ToLower(q.QueryType)
}

// validate reports why a query cannot be run, or nil when it can.
func (q queryModel) validate() error {
	switch q.queryType() {
	case QueryTypeFind, QueryTypeAggregate, QueryTypeCount, QueryTypeDistinct:
	default:
		return fmt.Errorf("unknown query type %q, expected one of %s, %s, %s or %s",
			q.QueryType, QueryTypeFind, QueryTypeAggregate, QueryTypeCount, QueryTypeDistinct)
	}

	if q.Database == "" {
		return fmt.Errorf("no database was specified")
	}
	if q.Collection == "" {
		return fmt.Errorf("no collection was specified")
	}

	return nil
}

// timeRange is the dashboard time range, in UNIX milliseconds.
type timeRange struct {
	from float64
	to   float64
}

func newTimeRange(r backend.TimeRange) timeRange {
	return timeRange{
		from: float64(r.From.UnixNano()) / float64(time.Millisecond),
		to:   float64(r.To.UnixNano()) / float64(time.Millisecond),
	}
}

// runQuery dispatches a query to the matching MongoDB operation.
func runQuery(ctx context.Context, collection *mongo.Collection, qm queryModel, tr timeRange) backend.DataResponse {
	switch qm.queryType() {
	case QueryTypeFind:
		return runFind(ctx, collection, qm, tr)
	case QueryTypeAggregate:
		return runAggregate(ctx, collection, qm, tr)
	case QueryTypeCount:
		return runCount(ctx, collection, qm, tr)
	case QueryTypeDistinct:
		return runDistinct(ctx, collection, qm, tr)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf(
			"unknown query type %q, expected one of %s, %s, %s or %s",
			qm.QueryType, QueryTypeFind, QueryTypeAggregate, QueryTypeCount, QueryTypeDistinct))
	}
}

// runFind executes a find, with its optional projection, sort, limit and skip.
func runFind(ctx context.Context, collection *mongo.Collection, qm queryModel, tr timeRange) backend.DataResponse {
	filter, err := parseDocument(qm.QueryText)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query unmarshal: %v", err.Error()))
	}

	findOptions := mongoOptions.Find()

	if text := cleanup(qm.Projection); text != "" {
		projection, err := parseOrderedDocument(text)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("projection unmarshal: %v", err.Error()))
		}
		findOptions.SetProjection(projection)
	}

	if text := cleanup(qm.Sort); text != "" {
		// The sort has to keep the order of its keys, so it is decoded into a
		// bson.D rather than a map.
		sortDocument, err := parseOrderedDocument(text)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("sort unmarshal: %v", err.Error()))
		}
		findOptions.SetSort(sortDocument)
	}

	if qm.Limit < 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "the limit cannot be negative")
	}
	if qm.Limit > 0 {
		findOptions.SetLimit(qm.Limit)
	}

	if qm.Skip < 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "the skip cannot be negative")
	}
	if qm.Skip > 0 {
		findOptions.SetSkip(qm.Skip)
	}

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB find error: %v", err.Error()))
	}

	return documentsResponse(ctx, cursor, qm.TimestampField, tr)
}

// runAggregate executes an aggregation pipeline.
func runAggregate(ctx context.Context, collection *mongo.Collection, qm queryModel, tr timeRange) backend.DataResponse {
	pipeline, err := parsePipeline(qm.Pipeline)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("pipeline unmarshal: %v", err.Error()))
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB aggregate error: %v", err.Error()))
	}

	return documentsResponse(ctx, cursor, qm.TimestampField, tr)
}

// runCount returns how many documents match the filter, as a single value.
func runCount(ctx context.Context, collection *mongo.Collection, qm queryModel, tr timeRange) backend.DataResponse {
	filter, err := parseDocument(qm.QueryText)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query unmarshal: %v", err.Error()))
	}

	// There is no document list to filter afterwards, so the time range is
	// pushed into the query itself.
	filter = withTimeRange(filter, qm.TimestampField, tr)

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB count error: %v", err.Error()))
	}

	var response backend.DataResponse
	frame := data.NewFrame("response", data.NewField("count", nil, []int64{count}))
	response.Frames = append(response.Frames, frame)

	return response
}

// runDistinct returns the unique values of a field.
func runDistinct(ctx context.Context, collection *mongo.Collection, qm queryModel, tr timeRange) backend.DataResponse {
	field := strings.TrimSpace(qm.DistinctField)
	if field == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no field was specified to collect the distinct values of")
	}

	filter, err := parseDocument(qm.QueryText)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query unmarshal: %v", err.Error()))
	}

	filter = withTimeRange(filter, qm.TimestampField, tr)

	var values []any
	if err := collection.Distinct(ctx, field, filter).Decode(&values); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("MongoDB distinct error: %v", err.Error()))
	}

	texts := make([]string, 0, len(values))
	for _, value := range values {
		texts = append(texts, stringify(value))
	}
	sort.Strings(texts)

	var response backend.DataResponse
	frame := data.NewFrame("response", data.NewField(field, nil, texts))
	response.Frames = append(response.Frames, frame)

	return response
}

// documentsResponse drains a cursor and turns the documents into a frame.
func documentsResponse(ctx context.Context, cursor *mongo.Cursor, timestampField string, tr timeRange) backend.DataResponse {
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.DefaultLogger.Warn("failed to close the MongoDB cursor", "error", err)
		}
	}()

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("cursor all error: %v", err.Error()))
	}

	var response backend.DataResponse
	response.Frames = append(response.Frames, documentsToFrame(docs, timestampField, tr))

	return response
}

// documentsToFrame converts documents into a frame. Every field is returned as
// text, except the timestamp field which becomes a real time field so that
// Grafana can use it on a time axis. When a timestamp field is set, documents
// outside of the dashboard time range are dropped.
func documentsToFrame(docs []bson.M, timestampField string, tr timeRange) *data.Frame {
	hasTimestampField := timestampField != ""

	// Identify all unique fields, sorted so that the frame is stable between queries.
	fieldSet := make(map[string]struct{})
	for _, doc := range docs {
		for key := range doc {
			fieldSet[key] = struct{}{}
		}
	}

	fieldNames := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		fieldNames = append(fieldNames, key)
	}
	sort.Strings(fieldNames)

	filteredDocs := make([]bson.M, 0, len(docs))
	timestamps := make([]time.Time, 0, len(docs))
	for _, doc := range docs {
		if hasTimestampField {
			timestamp, ok := toEpochMillis(doc[timestampField])
			if !ok {
				continue // Skip this document
			}

			if timestamp < tr.from || timestamp > tr.to {
				continue // Skip this document
			}

			timestamps = append(timestamps, time.UnixMilli(int64(timestamp)).UTC())
		}

		filteredDocs = append(filteredDocs, doc)
	}

	frame := data.NewFrame("response")

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

	return frame
}

// withTimeRange restricts a filter to the dashboard time range.
//
// The timestamp is matched both as a number of milliseconds and as a date, so
// that it works whichever way the documents store it. BSON never compares a
// date against a number, so only the matching representation can hit.
func withTimeRange(filter bson.M, timestampField string, tr timeRange) bson.M {
	if timestampField == "" {
		return filter
	}

	from := int64(tr.from)
	to := int64(tr.to)

	within := bson.A{
		bson.M{timestampField: bson.M{"$gte": from, "$lte": to}},
		bson.M{timestampField: bson.M{
			"$gte": bson.NewDateTimeFromTime(time.UnixMilli(from)),
			"$lte": bson.NewDateTimeFromTime(time.UnixMilli(to)),
		}},
	}

	// The filter may already carry an $and, so both are combined rather than
	// overwriting one another.
	if len(filter) == 0 {
		return bson.M{"$or": within}
	}
	return bson.M{"$and": bson.A{filter, bson.M{"$or": within}}}
}

// parseDocument reads a filter document written as MongoDB extended JSON. An
// empty text means "match everything".
func parseDocument(text string) (bson.M, error) {
	cleaned := cleanup(text)
	if cleaned == "" {
		return bson.M{}, nil
	}

	var document bson.M
	if err := bson.UnmarshalExtJSON([]byte(cleaned), false, &document); err != nil {
		return nil, err
	}
	return document, nil
}

// parseOrderedDocument reads a document while keeping the order of its keys,
// which matters for a sort.
func parseOrderedDocument(text string) (bson.D, error) {
	var document bson.D
	if err := bson.UnmarshalExtJSON([]byte(cleanup(text)), false, &document); err != nil {
		return nil, err
	}
	return document, nil
}

// parsePipeline reads an aggregation pipeline, written as an array of stages.
//
// Extended JSON only decodes documents, so the array is wrapped in one before
// being handed to the decoder.
func parsePipeline(text string) (bson.A, error) {
	cleaned := cleanup(text)
	if cleaned == "" {
		return bson.A{}, nil
	}

	if !strings.HasPrefix(cleaned, "[") {
		return nil, fmt.Errorf("a pipeline has to be an array of stages, for example [{\"$match\": {}}]")
	}

	var wrapper struct {
		Pipeline bson.A `bson:"pipeline"`
	}
	if err := bson.UnmarshalExtJSON([]byte(`{"pipeline": `+cleaned+`}`), false, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Pipeline, nil
}

// cleanup strips the comments and the surrounding spaces of a query input.
func cleanup(text string) string {
	return strings.TrimSpace(removeComments(text))
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
