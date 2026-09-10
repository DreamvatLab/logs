package mongodb

import (
	"context"
	"regexp"
	"sort"
	"time"

	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xtask"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// var (
// 	_clients     map[string]*logs.LogClient
// 	_cacheLocker = new(sync.RWMutex)
// )

// const (
// 	_CLIENT_DB    = "LogClients"
// 	_CLIENT_TABLE = "clients"
// )

var (
	_client *mongo.Client
	// _clientTable *mongo.Collection
)

type MongoDAL struct {
}

func Init() {
	connStr := core.ServiceConfigProvider.GetString("ConnectionStrings.MongoDB")
	// Create a new client and connect to the server.
	// v2 dropped the context parameter: Connect no longer does any I/O, the
	// connection is established lazily on the first operation.
	var err error
	_client, err = mongo.Connect(options.Client().ApplyURI(connStr))
	xerr.FatalIfErr(err)

	// _clientTable = _client.Database(_CLIENT_DB).Collection(_CLIENT_TABLE)
	// unique := true

	// _, err = _clientTable.Indexes().CreateMany(ctx, []mongo.IndexModel{
	// 	{
	// 		Keys:    bson.D{{Key: "id", Value: 1}},
	// 		Options: &options.IndexOptions{Unique: &unique},
	// 	},
	// })
	xerr.FatalIfErr(err)

	// err = refreshCache()
	// xerr.FatalIfErr(err)
}

// ************************************************************************************************

// func refreshCache() error {
// 	c, err := _clientTable.Find(nil, bson.M{})
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	var clients []*logs.LogClient
// 	c.All(nil, &clients)

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clients = make(map[string]*logs.LogClient, len(clients))
// 	for _, x := range clients {
// 		_clients[x.ID] = x
// 	}

// 	return nil
// }

// func (o *MongoDAL) InsertClient(client *logs.LogClient) error {
// 	_, err := _clientTable.InsertOne(nil, client)
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clients[client.ID] = client

// 	return nil
// }
// func (o *MongoDAL) GetClient(id string) (r *logs.LogClient, err error) {
// 	var ok bool
// 	_cacheLocker.RLock()
// 	r, ok = _clients[id]
// 	_cacheLocker.RUnlock()
// 	if ok {
// 		return r, nil
// 	}

// 	rs := _clientTable.FindOne(nil, bson.M{"id": id})
// 	err = rs.Err()
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			return nil, nil
// 		}
// 		return nil, xerr.WithStack(err)
// 	}

// 	err = rs.Decode(&r)
// 	if err != nil {
// 		return nil, xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	_clients[id] = r
// 	_cacheLocker.Unlock()

// 	return r, nil
// }
// func (o *MongoDAL) UpdateClient(client *logs.LogClient) error {
// 	_, err := _clientTable.UpdateOne(nil, bson.M{"id": client.ID}, bson.M{"$set": bson.M{
// 		"dbpolicy": client.DBPolicy,
// 	}})
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clients[client.ID] = client

// 	return nil
// }
// func (o *MongoDAL) DeleteClient(id string) error {
// 	_, err := _clientTable.DeleteOne(nil, bson.M{"id": id})
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	delete(_clients, id)

// 	return nil
// }
// func (o *MongoDAL) GetClients(query *logs.LogClientsQuery) ([]*logs.LogClient, error) {
// 	c, err := _clientTable.Find(nil, bson.M{})
// 	if err != nil {
// 		return nil, xerr.WithStack(err)
// 	}

// 	var clients []*logs.LogClient
// 	err = c.All(nil, &clients)
// 	if err != nil {
// 		return nil, xerr.WithStack(err)
// 	}

// 	return clients, nil
// }
// func (o *MongoDAL) RefreshCache() error {
// 	return refreshCache()
// }

// ************************************************************************************************

func (o *MongoDAL) GetDatabases(clientID string) ([]string, error) {
	return _client.ListDatabaseNames(
		context.Background(),
		// bson.M{"name": bson.M{"$regex": "LOG_" + clientID, "$options": "i"}},
		bson.M{"name": bson.M{"$regex": core.LOG_DB_PREFIX + clientID}},
		options.ListDatabases().SetNameOnly(true),
	)
}

func (o *MongoDAL) GetTables(database string) ([]string, error) {
	db := _client.Database(database)
	tables, err := db.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	for _, x := range tables {
		go func(dbName string) {
			table := db.Collection(dbName)
			_, err = table.Indexes().CreateOne(context.Background(), mongo.IndexModel{
				// Compound, and ordered: it has to match the $sort in GetLogEntries
				// key for key, otherwise that sort cannot use this index.
				Keys: bson.D{{Key: "createdonutc", Value: -1}, {Key: "_id", Value: -1}},
			})
			xerr.LogError(err)
		}(x)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(tables)))

	return tables, nil
}

func (o *MongoDAL) InsertLogEntry(dbName, tableName string, logEntry *logs.LogEntry) error {
	table := _client.Database(dbName).Collection(tableName)
	_, err := table.InsertOne(context.Background(), logEntry)
	if err != nil {
		return xerr.WithStack(err)
	}

	return nil
}

func (o *MongoDAL) GetLogEntry(query *logs.LogEntryQuery) (*logs.LogEntry, error) {
	table := _client.Database(query.DBName).Collection(query.TableName)

	rs := table.FindOne(context.Background(), bson.M{"id": query.ID})
	err := rs.Err()
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	var r *logs.LogEntry
	err = rs.Decode(&r)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}

// containsIgnoreCase builds a case-insensitive **substring** match.
//
// The user's input is escaped, i.e. taken literally rather than as a regular
// expression. It used to be passed straight into $regex, which silently
// returned nothing whenever the text contained a regex metacharacter — and log
// lines are full of them: searching for `批次#2913 [IBKR:DUM780185]` matched
// zero rows because `[...]` was parsed as a character class (2026-09-10).
// `.` `(` `)` `*` `+` `?` `|` `$` had the same effect, only less visibly: they
// widened the match instead of killing it.
//
// Note this drops the ability to search by regex. Nobody was using it on
// purpose — the boxes are labelled "User" / "TraceNo" / "Message", not "regex" —
// and a search box that quietly returns nothing is worse than one that cannot
// do character classes.
func containsIgnoreCase(v string) bson.M {
	return bson.M{"$regex": regexp.QuoteMeta(v), "$options": "i"}
}

func (o *MongoDAL) GetLogEntries(query *logs.LogEntriesQuery) ([]*logs.LogEntry, int64, error) {
	table := _client.Database(query.DBName).Collection(query.TableName)
	// Find
	match := bson.M{"$match": bson.M{}}
	matchExp := match["$match"].(bson.M)

	// if query.Keyword != "" {
	// 	matchExp["$or"] = []bson.M{
	// 		{"message": bson.M{"$regex": query.Keyword, "$options": "i"}},
	// 		{"error": bson.M{"$regex": query.Keyword, "$options": "i"}},
	// 	}
	// }

	// Build the time range into a single condition object. Assigning
	// matchExp["createdonutc"] twice would make the end time replace the start time.
	timeCond := bson.M{}
	if query.StartTime != "" {
		t, err := time.ParseInLocation(time.RFC3339, query.StartTime, time.UTC)
		if err != nil {
			return nil, 0, xerr.WithStack(err)
		}
		timeCond["$gte"] = t.UnixMilli()
	}
	if query.EndTime != "" {
		t, err := time.ParseInLocation(time.RFC3339, query.EndTime, time.UTC)
		if err != nil {
			return nil, 0, xerr.WithStack(err)
		}
		timeCond["$lte"] = t.UnixMilli()
	}
	if len(timeCond) > 0 {
		matchExp["createdonutc"] = timeCond
	}
	if query.Level >= 0 {
		matchExp["level"] = bson.M{"$eq": query.Level}
	}

	if query.User != "" {
		matchExp["user"] = containsIgnoreCase(query.User)
	}
	if query.TraceNo != "" {
		matchExp["traceno"] = containsIgnoreCase(query.TraceNo)
	}
	if query.Message != "" {
		matchExp["message"] = containsIgnoreCase(query.Message)
	}

	// TotalCount
	count := bson.M{"$count": "totalcount"}

	// Sort by timestamp, with _id only as a tiebreaker. _id is a Sonyflake hex
	// string generated on receipt, so it is not a reliable time order; the
	// tiebreaker keeps paging stable when timestamps collide.
	// bson.D, not bson.M: a map does not preserve key order, which would let
	// _id become the primary sort key at random.
	sortDir := -1
	sort := bson.M{"$sort": bson.D{{Key: "createdonutc", Value: sortDir}, {Key: "_id", Value: sortDir}}}

	// Paginate
	limit := bson.M{"$limit": query.PageSize}
	// Skip
	skip := bson.M{"$skip": (query.PageIndex - 1) * query.PageSize}

	// Replace existing implementation with new ParallelRun function
	results := xtask.ParallelRun(2,
		func() (interface{}, error) {
			countMapPtr := make(map[string]int64)
			var rs *mongo.Cursor
			rs, err := table.Aggregate(context.Background(), []bson.M{match, count})
			if err != nil {
				return nil, xerr.WithStack(err)
			}

			if rs.TryNext(context.Background()) {
				err = rs.Decode(&countMapPtr)
				if err != nil {
					return nil, xerr.WithStack(err)
				}
			}

			totalCount := countMapPtr["totalcount"]
			return totalCount, nil
		},
		func() (interface{}, error) {
			r := make([]*logs.LogEntry, 0, query.PageSize)
			var rs *mongo.Cursor
			rs, err := table.Aggregate(context.Background(), []bson.M{match, sort, skip, limit})
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			err = rs.All(context.Background(), &r)
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			return r, nil
		},
	)

	// Merge errors using xutils.JointErrors
	err := xerr.JointErrors(results[0].Error, results[1].Error)
	if err != nil {
		return nil, 0, err
	}

	// Get results
	totalCount := results[0].Result.(int64)
	list := results[1].Result.([]*logs.LogEntry)

	return list, totalCount, nil
}
