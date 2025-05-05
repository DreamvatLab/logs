package svc

import (
	"context"

	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/logs"
)

type LogClientService struct{}

func (o *LogClientService) GetClients(ctx context.Context, in *logs.LogClientsQuery) (*logs.LogClientsResult, error) {
	r := new(logs.LogClientsResult)

	list, err := _clientDAL.GetClients(in)
	if xerr.LogError(err) {
		r.Message = err.Error()
	}
	r.LogClients = make([]string, 0, len(list))
	for _, x := range list {
		r.LogClients = append(r.LogClients, x.ID)
	}
	return r, nil
}
func (o *LogClientService) GetDatabases(ctx context.Context, in *logs.DatabasesQuery) (*logs.DatabasesResult, error) {
	r := new(logs.DatabasesResult)

	list, err := _logDAL.GetDatabases(in.ClientID)
	if xerr.LogError(err) {
		r.Message = err.Error()
	}

	r.Databases = list
	return r, nil
}
func (o *LogClientService) GetTables(ctx context.Context, in *logs.TablesQuery) (*logs.TablesResult, error) {
	r := new(logs.TablesResult)
	list, err := _logDAL.GetTables(in.Database)
	if xerr.LogError(err) {
		r.Message = err.Error()
	}

	r.Tables = list
	return r, nil
}
