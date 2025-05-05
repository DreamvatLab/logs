package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DreamvatLab/go/xconv"
	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xtask"
	"github.com/DreamvatLab/go/xutils"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"

	"github.com/go-sql-driver/mysql"
	// _ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// const (
// 	_TIME_FORMAT = "2006-01-02T15:04:05Z"
// )

var (
	// _clientsMap map[string]*logs.LogClient
	_wherePool = &sync.Pool{
		New: func() any {
			return new(strings.Builder)
		},
	}
	// _cacheLocker = new(sync.RWMutex)
	_dbLocker = new(sync.RWMutex)
	_db       *sqlx.DB
)

func Init() {
	connStr := core.ServiceConfigProvider.GetString("ConnectionStrings.MySql")
	var err error
	_db, err = sqlx.Connect("mysql", connStr)
	xerr.FatalIfErr(err)

	connMaxLifetime := core.ServiceConfigProvider.GetInt("DataAccess.ConnMaxLifetime")
	maxOpenConns := core.ServiceConfigProvider.GetInt("DataAccess.MaxOpenConns")
	maxIdleConns := core.ServiceConfigProvider.GetInt("DataAccess.MaxIdleConns")

	_db.SetConnMaxLifetime(time.Second * time.Duration(connMaxLifetime))
	_db.SetMaxOpenConns(maxOpenConns)
	_db.SetMaxIdleConns(maxIdleConns)

	// err = refreshCache()
	// xerr.FatalIfErr(err)
}

type MySqlDAL struct {
}

// ************************************************************************************************

// func refreshCache() error {
// 	var clients []*logs.LogClient
// 	err := _db.Select(&clients, "SELECT * FROM `Clients`")
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clientsMap = make(map[string]*logs.LogClient, len(clients))
// 	for _, x := range clients {
// 		_clientsMap[x.ID] = x
// 	}

// 	return nil
// }

// func (o *MySqlDAL) InsertClient(client *logs.LogClient) error {
// 	_, err := _db.NamedExec("INSERT INTO `Clients`(`ID`,`DBPolicy`) VALUES(:ID, :DBPolicy)", client)
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clientsMap[client.ID] = client
// 	return nil
// }
// func (o *MySqlDAL) GetClient(id string) (r *logs.LogClient, err error) {
// 	var ok bool
// 	_cacheLocker.RLock()
// 	r, ok = _clientsMap[id]
// 	_cacheLocker.RUnlock()
// 	if ok {
// 		return r, nil
// 	}

// 	r = new(logs.LogClient)
// 	err = _db.Get(r, "SELECT * FROM `Clients` WHERE ID = ?", id)
// 	if err != nil && !errors.Is(err, sql.ErrNoRows) {
// 		return nil, xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	_clientsMap[id] = r
// 	_cacheLocker.Unlock()

// 	return r, nil
// }
// func (o *MySqlDAL) UpdateClient(client *logs.LogClient) error {
// 	_, err := _db.Exec("UPDATE `Clients` SET `DBPolicy` = ?", client.DBPolicy)
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	_clientsMap[client.ID] = client

// 	return nil
// }
// func (o *MySqlDAL) DeleteClient(id string) error {
// 	_, err := _db.Exec("DELETE FROM `Clients` WHERE `ID` = ?", id)
// 	if err != nil {
// 		return xerr.WithStack(err)
// 	}

// 	_cacheLocker.Lock()
// 	defer _cacheLocker.Unlock()
// 	delete(_clientsMap, id)

// 	return nil
// }
// func (o *MySqlDAL) GetClients(query *logs.LogClientsQuery) ([]*logs.LogClient, error) {
// 	listSel := "SELECT * FROM `Clients`"

// 	var r []*logs.LogClient
// 	err := _db.Select(&r, listSel)
// 	if err != nil {
// 		return nil, xerr.WithStack(err)
// 	}

// 	return r, nil
// }
// func (o *MySqlDAL) RefreshCache() error {
// 	return nil
// }

// ************************************************************************************************

func ensureDBTableExsits(err error, dbName, tableName string) error {
	var sql string
	if err != nil {
		if innerErr, ok := err.(*mysql.MySQLError); ok && innerErr.Number == 1146 { // 1146: Table not exists
			_dbLocker.Lock()
			defer _dbLocker.Unlock()
			// Select db
			sql = fmt.Sprintf(_SQL_USE_DB, dbName)
			_, err = _db.Exec(sql)
			if err != nil {
				if innerErr, ok := err.(*mysql.MySQLError); ok && innerErr.Number == 1049 { // 1049: Unknown db
					// Create db
					sql = fmt.Sprintf(_SQL_CREATE_DB, dbName)
					_, err = _db.Exec(sql)
					if err != nil {
						return xerr.WithStack(err)
					}
					// Create count SP
					sql = fmt.Sprintf(_SQL_SP_COUNT, dbName)
					_, err = _db.Exec(sql)
					if err != nil {
						return xerr.WithStack(err)
					}
					// Create page SP
					sql = fmt.Sprintf(_SQL_SP_PAGE, dbName)
					_, err = _db.Exec(sql)
					if err != nil {
						return xerr.WithStack(err)
					}
				} else {
					return xerr.WithStack(err)
				}
			}

			// Create table
			sql = fmt.Sprintf(_SQL_CREATE_TABLE, dbName, tableName, tableName, tableName, tableName, tableName, tableName, tableName)
			_, err = _db.Exec(sql)
			if err != nil {
				return xerr.WithStack(err)
			}
		} else {
			return xerr.WithStack(err)
		}
	}

	return nil
}

func (o *MySqlDAL) InsertLogEntry(dbName, tableName string, logEntry *logs.LogEntry) error {
	if logEntry == nil {
		return xerr.New("logEntry cannot be nil")
	}

	sql := fmt.Sprintf(_SQL_INSERT, dbName, tableName)
	_dbLocker.RLock()
	_, err := _db.NamedExec(sql, logEntry)
	_dbLocker.RUnlock()

	if err != nil {
		err = ensureDBTableExsits(err, dbName, tableName) // Ensure db and table are exist
		if err == nil {
			// No error, retry
			_, err := _db.NamedExec(sql, logEntry)
			if err != nil {
				return xerr.WithStack(err)
			}

		} else {
			return err
		}
	}

	return nil
}

func (o *MySqlDAL) GetLogEntry(query *logs.LogEntryQuery) (*logs.LogEntry, error) {
	if query == nil || query.ID == "" || query.DBName == "" {
		return nil, xerr.New("query, ID and DBName cannot be nil or empty")
	}

	_dbLocker.RLock()
	defer _dbLocker.RUnlock()

	r := new(logs.LogEntry)
	sqlSel := fmt.Sprintf(_SQL_SELECT_ONE, query.DBName, query.TableName)
	err := _db.Get(r, sqlSel, query.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, xerr.New("log entry not found")
		}
		return nil, xerr.WithStack(err)
	}

	return r, nil
}

func (o *MySqlDAL) GetLogEntries(query *logs.LogEntriesQuery) ([]*logs.LogEntry, int64, error) {
	if query == nil || query.DBName == "" || query.TableName == "" {
		return nil, 0, xerr.New("query, DBName and TableName cannot be nil or empty")
	}

	if query.PageIndex < 1 {
		return nil, 0, xerr.New("page index must be greater than 0")
	}

	if query.PageSize < 1 {
		return nil, 0, xerr.New("page size must be greater than 0")
	}

	// Build where
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
			countSql := fmt.Sprintf("CALL `%s`.`SYSSP_GetTotalCount` (?,?)", query.DBName)
			table := fmt.Sprintf("`%s`.`%s`", query.DBName, query.TableName)
			err := _db.Get(&totalCount, countSql, table, where.String())
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			return totalCount, nil
		},
		func() (interface{}, error) {
			listSql := fmt.Sprintf("CALL `%s`.`SYSSP_GetPagedData` (?,?,?,?,?,?)", query.DBName)
			table := fmt.Sprintf("`%s`.`%s`", query.DBName, query.TableName)
			ord := "`CreatedOnUtc` DESC"

			var r []*logs.LogEntry
			err := _db.Select(&r, listSql, query.PageSize, query.PageIndex, table, ord, "*", where.String())
			if err != nil {
				return nil, xerr.WithStack(err)
			}
			if r == nil {
				r = make([]*logs.LogEntry, 0)
			}
			return r, nil
		},
	)

	// Merge errors
	err := xutils.JointErrors(results[0].Error, results[1].Error)
	if err != nil {
		return nil, 0, err
	}

	totalCount := results[0].Result.(int64)
	list := results[1].Result.([]*logs.LogEntry)
	return list, totalCount, nil
}

func (o *MySqlDAL) GetDatabases(clientID string) ([]string, error) {
	sqlStr := "SELECT `schema_name` FROM information_schema.schemata WHERE SCHEMA_NAME LIKE ? ORDER BY `schema_name` DESC;"

	var r []string
	keyword := "LOG\\_" + clientID + "%"
	err := _db.Select(&r, sqlStr, keyword)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}

func (o *MySqlDAL) GetTables(database string) ([]string, error) {
	sqlStr := "SELECT `table_name` FROM information_schema.tables WHERE table_schema = ? AND table_type = 'base table' ORDER BY `table_name` DESC;"

	var r []string
	err := _db.Select(&r, sqlStr, database)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	return r, nil
}
