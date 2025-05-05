package dal

import (
	"github.com/DreamvatLab/go/xlog"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"
	"github.com/DreamvatLab/logs/host/dal/ch"
	"github.com/DreamvatLab/logs/host/dal/mongodb"
	"github.com/DreamvatLab/logs/host/dal/mysql"
	"github.com/DreamvatLab/logs/host/dal/redis"
)

type ILogDAL interface {
	InsertLogEntry(dbName, tableName string, logEntry *logs.LogEntry) error
	GetLogEntry(query *logs.LogEntryQuery) (*logs.LogEntry, error)
	GetLogEntries(query *logs.LogEntriesQuery) ([]*logs.LogEntry, int64, error)
	GetDatabases(clientID string) ([]string, error)
	GetTables(database string) ([]string, error)
}

type IClientDAL interface {
	InsertClient(*logs.LogClient) error
	GetClient(id string) (*logs.LogClient, error)
	UpdateClient(*logs.LogClient) error
	DeleteClient(id string) error
	GetClients(in *logs.LogClientsQuery) ([]*logs.LogClient, error)
}

func NewLogDAL() ILogDAL {
	provider := core.ServiceConfigProvider.GetStringDefault("DataAccess.Provider", "mongodb")
	if provider == "clickhouse" {
		ch.Init()
		r := new(ch.ClickHouseDAL)
		return r
	} else if provider == "mongodb" {
		mongodb.Init()
		r := new(mongodb.MongoDAL)
		return r
	} else if provider == "mysql" {
		mysql.Init()
		r := new(mysql.MySqlDAL)
		return r
	}

	xlog.Fatalf("Provider '%s' is not supported", provider)
	return nil
}

func NewClientDAL() IClientDAL {
	redisDAL := new(redis.RedisDAL)
	redisDAL.Init(core.ServiceConfigProvider)
	return redisDAL
}
