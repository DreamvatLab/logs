package mongodb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DreamvatLab/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Time range regression test for GetLogEntries.
//
// Before the fix, matchExp["createdonutc"] was assigned once per bound, so
// supplying both bounds made the end time replace the start time and the query
// silently degraded to "$lte only".
//
// Requires a reachable MongoDB holding log data; skipped when there is none.
// Override the connection with LOGS_TEST_MONGO.

const (
	testDBName    = "LOG_HUBAPI"
	testTableName = "2026"
	testStartTime = "2026-09-06T00:00:00Z"
	testEndTime   = "2026-09-07T00:00:00Z"
)

func connectTestClient(t *testing.T) {
	t.Helper()

	connStr := os.Getenv("LOGS_TEST_MONGO")
	if connStr == "" {
		connStr = "mongodb://sa:Famous901@localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// v2's Connect does no I/O, so it only fails on a bad URI; reachability is
	// what the Ping below decides.
	c, err := mongo.Connect(options.Client().ApplyURI(connStr))
	if err != nil {
		t.Skipf("cannot build a MongoDB client, skipping: %v", err)
	}
	if err = c.Ping(ctx, nil); err != nil {
		t.Skipf("cannot ping MongoDB, skipping: %v", err)
	}

	_client = c
	t.Cleanup(func() { _client.Disconnect(context.Background()) })

	n, err := c.Database(testDBName).Collection(testTableName).EstimatedDocumentCount(ctx)
	if err != nil || n == 0 {
		t.Skipf("%s.%s is empty or unreadable, skipping", testDBName, testTableName)
	}
}

// expectedCount counts documents straight through the driver, bypassing the DAL,
// so the expectation never inherits the bug being tested.
func expectedCount(t *testing.T, cond bson.M) int64 {
	t.Helper()

	filter := bson.M{}
	if len(cond) > 0 {
		filter["createdonutc"] = cond
	}

	n, err := _client.Database(testDBName).Collection(testTableName).CountDocuments(context.Background(), filter)
	if err != nil {
		t.Fatalf("CountDocuments failed: %v", err)
	}

	return n
}

func mustParseMilli(t *testing.T, v string) int64 {
	t.Helper()

	parsed, err := time.ParseInLocation(time.RFC3339, v, time.UTC)
	if err != nil {
		t.Fatalf("cannot parse %q: %v", v, err)
	}

	return parsed.UnixMilli()
}

func queryFor(startTime, endTime string) *logs.LogEntriesQuery {
	return &logs.LogEntriesQuery{
		DBName:    testDBName,
		TableName: testTableName,
		StartTime: startTime,
		EndTime:   endTime,
		Level:     -1, // -1 disables the level filter
		PageIndex: 1,
		PageSize:  50,
	}
}

func TestGetLogEntriesTimeRange(t *testing.T) {
	connectTestClient(t)

	startMilli := mustParseMilli(t, testStartTime)
	endMilli := mustParseMilli(t, testEndTime)

	cases := []struct {
		name      string
		startTime string
		endTime   string
		cond      bson.M
	}{
		{"StartOnly", testStartTime, "", bson.M{"$gte": startMilli}},
		{"EndOnly", "", testEndTime, bson.M{"$lte": endMilli}},
		{"BothBounds", testStartTime, testEndTime, bson.M{"$gte": startMilli, "$lte": endMilli}},
		{"NoBounds", "", "", nil},
	}

	dal := new(MongoDAL)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := expectedCount(t, c.cond)

			_, totalCount, err := dal.GetLogEntries(queryFor(c.startTime, c.endTime))
			if err != nil {
				t.Fatalf("GetLogEntries failed: %v", err)
			}

			if totalCount != want {
				t.Errorf("totalCount = %d, want %d", totalCount, want)
			}
		})
	}
}

// TestGetLogEntriesBothBoundsRespectsStart is the direct regression guard: every
// returned entry has to satisfy both bounds, not just the end one.
func TestGetLogEntriesBothBoundsRespectsStart(t *testing.T) {
	connectTestClient(t)

	startMilli := mustParseMilli(t, testStartTime)
	endMilli := mustParseMilli(t, testEndTime)

	entries, totalCount, err := dalGetAll(t, queryFor(testStartTime, testEndTime))
	if err != nil {
		t.Fatalf("GetLogEntries failed: %v", err)
	}

	endOnly := expectedCount(t, bson.M{"$lte": endMilli})
	if totalCount == endOnly && totalCount != expectedCount(t, bson.M{"$gte": startMilli, "$lte": endMilli}) {
		t.Fatalf("start time was dropped: totalCount = %d matches the $lte-only count", totalCount)
	}

	for _, e := range entries {
		if e.CreatedOnUtc < startMilli || e.CreatedOnUtc > endMilli {
			t.Errorf("entry %s createdonutc = %d is outside [%d, %d]", e.ID, e.CreatedOnUtc, startMilli, endMilli)
		}
	}
}

func dalGetAll(t *testing.T, query *logs.LogEntriesQuery) ([]*logs.LogEntry, int64, error) {
	t.Helper()

	dal := new(MongoDAL)
	query.PageSize = 1000

	return dal.GetLogEntries(query)
}
