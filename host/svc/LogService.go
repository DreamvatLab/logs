package svc

import (
	"context"
	"fmt"

	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xutils"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"
)

type LogService struct{}

func (o *LogService) WriteLogEntry(_ context.Context, in *logs.WriteLogCommand) (*logs.LogEntryResult, error) {
	r := new(logs.LogEntryResult)

	if _asyncWriting {
		go func() {
			err := write(in)
			xerr.LogError(err)
		}()
	} else {
		err := write(in)
		if xerr.LogError(err) {
			r.Message = err.Error()
		}
	}

	return r, nil
}

func write(in *logs.WriteLogCommand) error {
	client, err := _clientDAL.GetClient(in.ClientID)
	if err != nil {
		return err
	} else if client == nil {
		return xerr.Errorf("Client '%s' not found", in.ClientID)
	} else {
		if in.LogEntry.Level < client.Level {
			// skip
			return nil
		}

		// determine database and table
		// createdOnUtc := time.UnixMilli(in.LogEntry.CreatedOnUtc)
		createdOnUtc := in.LogEntry.CreatedOnUtc.AsTime()

		var dbName, tableName string
		switch client.DBPolicy {
		case 1: // By Year
			dbName = fmt.Sprintf("%s%s_%04d", core.LOG_DB_PREFIX, client.ID, createdOnUtc.Year())
			// Use month as table name
			tableName = fmt.Sprintf("%02d", createdOnUtc.Month())
		case 2: // By Month
			dbName = fmt.Sprintf("%s%s_%04d%02d", core.LOG_DB_PREFIX, client.ID, createdOnUtc.Year(), createdOnUtc.Month())
			// Use day as table name
			tableName = fmt.Sprintf("%02d", createdOnUtc.Day())
		case 3: // By Day
			dbName = fmt.Sprintf("%s%s_%04d%02d%02d", core.LOG_DB_PREFIX, client.ID, createdOnUtc.Year(), createdOnUtc.Month(), createdOnUtc.Day())
			// Use hour as table name
			tableName = fmt.Sprintf("%02d", createdOnUtc.Hour())
		default:
			dbName = fmt.Sprintf("%s%s", core.LOG_DB_PREFIX, client.ID)
			// Use year as table name
			tableName = fmt.Sprintf("%02d", createdOnUtc.Year())
		}
		// generate id
		in.LogEntry.ID = xutils.GenerateStringID()

		err = _logDAL.InsertLogEntry(dbName, tableName, in.LogEntry)
		return err
	}
}

func (o *LogService) GetLogEntry(_ context.Context, query *logs.LogEntryQuery) (*logs.LogEntryResult, error) {
	r := new(logs.LogEntryResult)

	_logDAL.GetLogEntry(query)

	return r, nil
}
func (o *LogService) GetLogEntries(_ context.Context, query *logs.LogEntriesQuery) (*logs.LogEntriesResult, error) {
	r := new(logs.LogEntriesResult)
	var err error
	r.LogEntries, r.TotalCount, err = _logDAL.GetLogEntries(query)
	if xerr.LogError(err) {
		r.Message = err.Error()
	}

	return r, nil
}
