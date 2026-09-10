package mongodb

import (
	"context"
	"strings"
	"testing"

	"github.com/DreamvatLab/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Ordering and paging regression tests for GetLogEntries.
//
// Before the fix, $sort used _id, a Sonyflake hex string generated when the host
// received the entry rather than when it was logged, so the list order did not
// follow createdonutc. With no tiebreaker, $skip/$limit paging over colliding
// timestamps could also repeat or drop rows.
//
// The window ends in the past so a live writer cannot shift rows mid-test.

const testOrderingEndTime = "2026-09-09T00:00:00Z"

// correctOrder is the expected order, read straight through the driver so the
// expectation never inherits the bug being tested.
func correctOrder(t *testing.T, endMilli int64) []*logs.LogEntry {
	t.Helper()

	cursor, err := _client.Database(testDBName).Collection(testTableName).Find(
		context.Background(),
		bson.M{"createdonutc": bson.M{"$lte": endMilli}},
		optionsSortByTimeThenID(),
	)
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	var r []*logs.LogEntry
	if err = cursor.All(context.Background(), &r); err != nil {
		t.Fatalf("cursor.All failed: %v", err)
	}

	return r
}

func idsOf(entries []*logs.LogEntry) []string {
	r := make([]string, 0, len(entries))
	for _, e := range entries {
		r = append(r, e.ID)
	}

	return r
}

func orderingQuery(pageIndex, pageSize int32) *logs.LogEntriesQuery {
	q := queryFor("", testOrderingEndTime)
	q.PageIndex = pageIndex
	q.PageSize = pageSize

	return q
}

// TestGetLogEntriesOrdering checks the returned order against a timestamp-first
// ordering. Sorting by _id puts rows in receipt order instead, which differs.
func TestGetLogEntriesOrdering(t *testing.T) {
	connectTestClient(t)

	endMilli := mustParseMilli(t, testOrderingEndTime)
	want := correctOrder(t, endMilli)
	if len(want) < 20 {
		t.Skipf("only %d entries in the test window, too few to be meaningful", len(want))
	}

	dal := new(MongoDAL)

	got, totalCount, err := dal.GetLogEntries(orderingQuery(1, int32(len(want))))
	if err != nil {
		t.Fatalf("GetLogEntries failed: %v", err)
	}

	if totalCount != int64(len(want)) {
		t.Fatalf("totalCount = %d, want %d", totalCount, len(want))
	}

	gotIDs, wantIDs := idsOf(got), idsOf(want)
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %d entries, want %d", len(gotIDs), len(wantIDs))
	}

	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("order differs at position %d: got %s (createdonutc %d), want %s (createdonutc %d)",
				i, gotIDs[i], got[i].CreatedOnUtc, wantIDs[i], want[i].CreatedOnUtc)
		}
	}
}

// TestGetLogEntriesDescendingByTimestamp is the direct guard on the primary sort
// key: every entry must be no newer than the one before it.
func TestGetLogEntriesDescendingByTimestamp(t *testing.T) {
	connectTestClient(t)

	endMilli := mustParseMilli(t, testOrderingEndTime)

	dal := new(MongoDAL)

	got, _, err := dal.GetLogEntries(orderingQuery(1, 1000))
	if err != nil {
		t.Fatalf("GetLogEntries failed: %v", err)
	}
	if len(got) < 20 {
		t.Skipf("only %d entries returned, too few to be meaningful", len(got))
	}

	inversions := 0
	for i := 1; i < len(got); i++ {
		if got[i].CreatedOnUtc > got[i-1].CreatedOnUtc {
			inversions++
			if inversions <= 5 {
				t.Errorf("inversion at position %d: %d (%s) is newer than %d (%s)",
					i, got[i].CreatedOnUtc, got[i].ID, got[i-1].CreatedOnUtc, got[i-1].ID)
			}
		}
	}

	if inversions > 0 {
		t.Errorf("%d inversions in %d entries (window ends %s)", inversions, len(got), testOrderingEndTime)
	}

	if endMilli < got[0].CreatedOnUtc {
		t.Errorf("first entry %d is past the window end %d", got[0].CreatedOnUtc, endMilli)
	}
}

// TestGetLogEntriesPagingIsCompleteAndUnique is what the tiebreaker actually
// buys: with colliding timestamps and no secondary key, $skip/$limit can repeat
// or drop rows. Only paging all the way through shows it; page one looks fine.
func TestGetLogEntriesPagingIsCompleteAndUnique(t *testing.T) {
	connectTestClient(t)

	endMilli := mustParseMilli(t, testOrderingEndTime)
	want := correctOrder(t, endMilli)
	if len(want) < 20 {
		t.Skipf("only %d entries in the test window, too few to be meaningful", len(want))
	}

	collisions := countTimestampCollisions(want)
	t.Logf("%d entries in window, %d of them share a millisecond with another entry", len(want), collisions)
	if collisions == 0 {
		t.Logf("warning: no colliding timestamps, this run does not exercise the tiebreaker")
	}

	const pageSize = 10

	dal := new(MongoDAL)
	seen := make(map[string]int, len(want))
	var paged []string

	for pageIndex := int32(1); ; pageIndex++ {
		entries, totalCount, err := dal.GetLogEntries(orderingQuery(pageIndex, pageSize))
		if err != nil {
			t.Fatalf("GetLogEntries page %d failed: %v", pageIndex, err)
		}

		if totalCount != int64(len(want)) {
			t.Fatalf("page %d: totalCount = %d, want %d", pageIndex, totalCount, len(want))
		}

		if len(entries) == 0 {
			break
		}

		for _, e := range entries {
			seen[e.ID]++
			paged = append(paged, e.ID)
		}

		if pageIndex > int32(len(want)/pageSize)+5 {
			t.Fatalf("paging did not terminate after %d pages", pageIndex)
		}
	}

	for id, n := range seen {
		if n > 1 {
			t.Errorf("entry %s was returned on %d different pages", id, n)
		}
	}

	if len(paged) != len(want) {
		t.Errorf("paging returned %d entries, want %d (%d unique)", len(paged), len(want), len(seen))
	}

	wantIDs := idsOf(want)
	for _, id := range wantIDs {
		if _, ok := seen[id]; !ok {
			t.Errorf("entry %s was never returned by any page", id)
		}
	}

	// Paging must also reproduce the same order as a single unpaged query.
	for i := range wantIDs {
		if i >= len(paged) {
			break
		}
		if paged[i] != wantIDs[i] {
			t.Fatalf("paged order differs at position %d: got %s, want %s", i, paged[i], wantIDs[i])
		}
	}
}

func countTimestampCollisions(entries []*logs.LogEntry) int {
	byMilli := make(map[int64]int, len(entries))
	for _, e := range entries {
		byMilli[e.CreatedOnUtc]++
	}

	r := 0
	for _, n := range byMilli {
		if n > 1 {
			r += n
		}
	}

	return r
}

// TestSortUsesCompoundIndex checks that the new $sort is served by an index scan
// rather than an in-memory sort, which is the reason the index is compound.
func TestSortUsesCompoundIndex(t *testing.T) {
	connectTestClient(t)

	table := _client.Database(testDBName).Collection(testTableName)

	// GetTables creates this index in a goroutine; create it here so the test
	// does not depend on that timing.
	if _, err := table.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "createdonutc", Value: -1}, {Key: "_id", Value: -1}},
	}); err != nil {
		t.Fatalf("cannot create the compound index: %v", err)
	}

	explained := explainAggregate(t, table)
	if explained == "" {
		t.Skip("explain returned nothing usable on this server")
	}

	if !containsStage(explained, "IXSCAN") {
		t.Errorf("the sort is not served by an index scan; explain:\n%s", explained)
	}

	if containsStage(explained, "SORT") {
		t.Errorf("the winning plan still contains an in-memory SORT stage; plan:\n%s", explained)
	}

	if !containsStage(explained, "createdonutc_-1__id_-1") {
		t.Errorf("the winning plan does not use the compound index; plan:\n%s", explained)
	}
}

func optionsSortByTimeThenID() *options.FindOptionsBuilder {
	return options.Find().SetSort(bson.D{{Key: "createdonutc", Value: -1}, {Key: "_id", Value: -1}})
}

// explainAggregate runs the same pipeline shape GetLogEntries uses and returns
// the winning plan as a JSON string. Only the winning plan: rejectedPlans holds
// the alternatives the planner discarded, and one of those legitimately carries
// a SORT stage, which would otherwise look like a failure.
func explainAggregate(t *testing.T, table *mongo.Collection) string {
	t.Helper()

	endMilli := mustParseMilli(t, testOrderingEndTime)
	pipeline := []bson.M{
		{"$match": bson.M{"createdonutc": bson.M{"$lte": endMilli}}},
		{"$sort": bson.D{{Key: "createdonutc", Value: -1}, {Key: "_id", Value: -1}}},
		{"$skip": 0},
		{"$limit": 10},
	}

	cmd := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "aggregate", Value: table.Name()},
			{Key: "pipeline", Value: pipeline},
			{Key: "cursor", Value: bson.M{}},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}

	var raw bson.Raw
	if err := table.Database().RunCommand(context.Background(), cmd).Decode(&raw); err != nil {
		t.Logf("explain failed: %v", err)
		return ""
	}

	winning, err := raw.LookupErr("queryPlanner", "winningPlan")
	if err != nil {
		t.Logf("no queryPlanner.winningPlan in the explain output: %v", err)
		return ""
	}

	return winning.String()
}

func containsStage(explained, needle string) bool {
	return strings.Contains(explained, needle)
}
