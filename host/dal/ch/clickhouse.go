package ch

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/DreamvatLab/go/xconv"
	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xtask"
	"github.com/DreamvatLab/go/xutils"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"
	"github.com/jmoiron/sqlx"
)

var (
	_db        *sqlx.DB
	_wherePool = &sync.Pool{
		New: func() any {
			return new(strings.Builder)
		},
	}
	_dbLocker = new(sync.RWMutex)
)

func Init() {
	connStr := core.ServiceConfigProvider.GetString("ConnectionStrings.ClickHouse")

	var err error
	_db, err = sqlx.Connect("clickhouse", connStr)
	xerr.FatalIfErr(err)

	connMaxLifetime := core.ServiceConfigProvider.GetInt("DataAccess.ConnMaxLifetime")
	maxOpenConns := core.ServiceConfigProvider.GetInt("DataAccess.MaxOpenConns")
	maxIdleConns := core.ServiceConfigProvider.GetInt("DataAccess.MaxIdleConns")

	_db.SetConnMaxLifetime(time.Second * time.Duration(connMaxLifetime))
	_db.SetMaxOpenConns(maxOpenConns)
	_db.SetMaxIdleConns(maxIdleConns)
}

type ClickHouseDAL struct{}

func ensureDBTableExsits(err error, dbName, tableName string) error {
	var sql string
	if err != nil {
		if innerErr, ok := err.(*proto.Exception); ok {
			_dbLocker.Lock()
			defer _dbLocker.Unlock()
			if innerErr.Code == 81 { // 81: DB not exists
				// Create db
				sql = fmt.Sprintf(_SQL_CREATE_DB, dbName)
				_, err = _db.Exec(sql)
				if err != nil {
					return xerr.WithStack(err)
				}

				// Create table
				sql = fmt.Sprintf(_SQL_CREATE_TABLE, dbName, tableName)
				_, err = _db.Exec(sql)
				if err != nil {
					return xerr.WithStack(err)
				}
			} else if innerErr.Code == 60 { // 60: Table not exists
				// Create table only
				sql = fmt.Sprintf(_SQL_CREATE_TABLE, dbName, tableName)
				_, err = _db.Exec(sql)
				if err != nil {
					return xerr.WithStack(err)
				}
			}
		} else {
			return xerr.WithStack(err)
		}
	}

	return nil
}

func (o *ClickHouseDAL) InsertLogEntry(dbName, tableName string, logEntry *logs.LogEntry) error {
	if logEntry == nil {
		return xerr.New("logEntry cannot be nil")
	}

	sqlStr := fmt.Sprintf(_SQL_INSERT, dbName, tableName)
	_dbLocker.RLock()
	_, err := _db.Exec(sqlStr,
		logEntry.ID,
		logEntry.TraceNo,
		logEntry.User,
		logEntry.Message,
		logEntry.Error,
		logEntry.StackTrace,
		logEntry.Payload,
		int32(logEntry.Level),
		logEntry.Flags,
		logEntry.CreatedOnUtc,
	)
	_dbLocker.RUnlock()
	if err != nil {
		err = ensureDBTableExsits(err, dbName, tableName) // Ensure db and table are exist
		if err != nil {
			return xerr.WithStack(err)
		}

		// Retry
		_, err = _db.Exec(sqlStr,
			logEntry.ID,
			logEntry.TraceNo,
			logEntry.User,
			logEntry.Message,
			logEntry.Error,
			logEntry.StackTrace,
			logEntry.Payload,
			int32(logEntry.Level),
			logEntry.Flags,
			logEntry.CreatedOnUtc,
		)

		if err != nil {
			return xerr.WithStack(err)
		}
	}

	return nil
}
func (o *ClickHouseDAL) GetLogEntry(query *logs.LogEntryQuery) (*logs.LogEntry, error) {
	r := new(logs.LogEntry)

	sqlSel := fmt.Sprintf(_SQL_SELECT_ONE, query.DBName, query.TableName)

	_dbLocker.RLock()
	err := _db.Get(r, sqlSel, query.ID)
	_dbLocker.RUnlock()

	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}
func (o *ClickHouseDAL) GetLogEntries(query *logs.LogEntriesQuery) ([]*logs.LogEntry, int64, error) {
	if query == nil || query.DBName == "" || query.TableName == "" {
		return nil, 0, xerr.New("query, DBName and TableName cannot be nil or empty")
	}

	if query.PageIndex < 1 {
		return nil, 0, xerr.New("page index must be greater than 0")
	}

	if query.PageSize < 1 {
		return nil, 0, xerr.New("page size must be greater than 0")
	}

	where := _wherePool.Get().(*strings.Builder)
	defer func() {
		where.Reset()
		_wherePool.Put(where)
	}()

	if query.StartTime != "" {
		t, err := time.ParseInLocation(time.RFC3339, query.StartTime, time.UTC)
		if err != nil {
			return nil, 0, xerr.WithStack(err)
		}
		where.WriteString(" AND `CreatedOnUtc` >= " + xconv.ToString(t.UnixMilli()))
	}
	if query.EndTime != "" {
		t, err := time.ParseInLocation(time.RFC3339, query.EndTime, time.UTC)
		t = t.Add(time.Hour * 24)
		if err != nil {
			return nil, 0, xerr.WithStack(err)
		}
		where.WriteString(" AND `CreatedOnUtc` <= " + xconv.ToString(t.UnixMilli()))
	}
	if query.Level >= 0 {
		where.WriteString(" AND `Level` = " + strconv.FormatInt(int64(query.Level), 10))
	}

	// TODO: Prevent sql injection
	if query.User != "" {
		likeSql := " AND `User` LIKE '"
		if query.Flags&1 == 1 { // Has flag, do left & right fuzzy search, other wise, only do right fuzzy search
			likeSql += "%"
		}
		likeSql += query.User + "%'"
		where.WriteString(likeSql)
	}
	if query.TraceNo != "" {
		likeSql := " AND `TraceNo` LIKE '"
		if query.Flags&1 == 1 { // Has flag, do left & right fuzzy search, other wise, only do right fuzzy search
			likeSql += "%"
		}
		likeSql += query.TraceNo + "%'"
		where.WriteString(likeSql)
	}
	if query.Message != "" {
		likeSql := " AND `Message` LIKE '"
		if query.Flags&1 == 1 { // Has flag, do left & right fuzzy search, other wise, only do right fuzzy search
			likeSql += "%"
		}
		likeSql += query.Message + "%'"
		where.WriteString(likeSql)
	}

	_dbLocker.RLock()
	defer _dbLocker.RUnlock()

	// Parallel run
	results := xtask.ParallelRun(
		func() (interface{}, error) {
			var totalCount int64
			countSql := fmt.Sprintf("SELECT COUNT(0) FROM `%s`.`%s` WHERE 0 = 0 %s", query.DBName, query.TableName, where.String())
			err := _db.Get(&totalCount, countSql)
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			return totalCount, nil
		},
		func() (interface{}, error) {
			start := (query.PageIndex - 1) * query.PageSize
			end := start + query.PageSize

			listSql := fmt.Sprintf("SELECT * FROM `%s`.`%s` WHERE 0 = 0 %s ORDER BY `CreatedOnUtc` DESC LIMIT %d, %d", query.DBName, query.TableName, where.String(), start, end)

			var r []*logs.LogEntry
			err := _db.Select(&r, listSql)
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			if r == nil {
				r = make([]*logs.LogEntry, 0)
			}
			return r, nil
		},
	)

	// Process results
	if len(results) != 2 {
		return nil, 0, xerr.New("unexpected number of results")
	}

	// Merge errors using xutils.JointErrors
	err := xutils.JointErrors(results[0].Error, results[1].Error)
	if err != nil {
		return nil, 0, err
	}

	totalCount := results[0].Result.(int64)
	list := results[1].Result.([]*logs.LogEntry)
	return list, totalCount, nil
}
func (o *ClickHouseDAL) GetDatabases(clientID string) ([]string, error) {
	var r []string
	keyword := "LOG\\_" + clientID + "%"
	err := _db.Select(&r, _SQL_SELECT_DATABASES, keyword)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}
func (o *ClickHouseDAL) GetTables(database string) ([]string, error) {
	var r []string
	err := _db.Select(&r, _SQL_SELECT_TABLES, database)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}
