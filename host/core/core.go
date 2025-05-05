package core

import (
	"github.com/DreamvatLab/go/xconfig"
	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xlog"
)

const (
	LOG_DB_PREFIX = "LOG_"
)

var (
	WebConfigProvider     xconfig.IConfigProvider
	ServiceConfigProvider xconfig.IConfigProvider
)

func Init() {
	ServiceConfigProvider = xconfig.NewJsonConfigProvider("service.json")
	var logConfig *xlog.LogConfig
	err := ServiceConfigProvider.GetStruct("Log", &logConfig)
	xerr.FatalIfErr(err)
	xlog.Init(logConfig)

	WebConfigProvider = xconfig.NewJsonConfigProvider("web.json")
}
