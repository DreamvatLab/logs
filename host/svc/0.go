package svc

import (
	"github.com/DreamvatLab/logs/host/core"
	"github.com/DreamvatLab/logs/host/dal"
)

var (
	_logDAL       dal.ILogDAL
	_clientDAL    dal.IClientDAL
	_asyncWriting bool
)

func Init() {
	_asyncWriting = core.ServiceConfigProvider.GetBool("AsyncWriting")
	_logDAL = dal.NewLogDAL()
	_clientDAL = dal.NewClientDAL()
}
