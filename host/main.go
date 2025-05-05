package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	fp "path/filepath"
	"strings"

	"github.com/DreamvatLab/go/xbytes"
	"github.com/DreamvatLab/go/xconfig"
	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xhttp"
	"github.com/DreamvatLab/go/xlog"
	"github.com/DreamvatLab/logs"
	"github.com/DreamvatLab/logs/host/core"
	"github.com/DreamvatLab/logs/host/svc"
	"github.com/fasthttp/router"
	"github.com/hashicorp/consul/api"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
)

const (
	_filepath = "filepath"
	_suffix   = "/{" + _filepath + ":*}"
)

var (
	logService       *svc.LogService
	logClientService *svc.LogClientService
)

//go:embed wwwroot
var staticFiles embed.FS

// Custom logger implementation of fasthttp.Logger interface
type customLogger struct{}

func (o *customLogger) Printf(format string, args ...any) {
	xlog.Debugf(format, args...)
}

func main() {
	core.Init()
	svc.Init()

	logService = new(svc.LogService)
	logClientService = new(svc.LogClientService)

	grpcServer := grpc.NewServer()
	go func() {
		// Register to consul
		err := registerConsulServiceInfo(core.ServiceConfigProvider)
		xerr.FatalIfErr(err)

		// Register GRPC service
		logs.RegisterLogEntryServiceServer(grpcServer, logService)

		grpcServerListenAddr := core.ServiceConfigProvider.GetString("ListenAddr")
		lis, err := net.Listen("tcp", grpcServerListenAddr)
		if err != nil {
			xlog.Fatal(err)
		}

		xlog.Infof("GRPC server listen on %s", grpcServerListenAddr)
		xerr.FatalIfErr(grpcServer.Serve(lis))
	}()

	router := router.New()
	router.POST("/api/logs", getLogs)
	router.GET("/api/listData", getListData)

	serveEmbedFiles(router, _suffix, "wwwroot", staticFiles)

	allowedOrigin := core.WebConfigProvider.GetString("CORS.AllowedOrigin")
	allowedMethods := core.WebConfigProvider.GetString("CORS.AllowedMethods")
	allowedHeaders := core.WebConfigProvider.GetString("CORS.AllowedHeaders")

	// Create CORS middleware
	corsMiddleware := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if allowedOrigin != "" {
				ctx.Response.Header.Set("Access-Control-Allow-Origin", allowedOrigin)
			}
			if allowedMethods != "" {
				ctx.Response.Header.Set("Access-Control-Allow-Methods", allowedMethods)
			}
			if allowedHeaders != "" {
				ctx.Response.Header.Set("Access-Control-Allow-Headers", allowedHeaders)
			}

			// Handle preflight requests
			if string(ctx.Method()) == "OPTIONS" {
				ctx.SetStatusCode(fasthttp.StatusOK)
				return
			}

			next(ctx)
		}
	}

	webServer := &fasthttp.Server{
		Handler: corsMiddleware(router.Handler),
		Logger:  &customLogger{},
	}

	webServerListenAddr := core.WebConfigProvider.GetString("ListenAddr")
	xlog.Infof("Web server listen on %s", webServerListenAddr)

	xerr.FatalIfErr(webServer.ListenAndServe(webServerListenAddr))
}

func getLogs(ctx *fasthttp.RequestCtx) {
	var query *logs.LogEntriesQuery
	bodyData := ctx.Request.Body()
	err := json.Unmarshal(bodyData, &query)

	if handleErr(err, ctx) {
		return
	}

	if query.PageSize <= 0 || query.PageIndex < 1 || query.DBName == "" || query.TableName == "" {
		handleErr(err, ctx)
		return
	}

	rs, err := logService.GetLogEntries(context.Background(), query)
	if !handleErr(err, ctx) {
		jsonBytes, err := json.Marshal(rs)
		if !handleErr(err, ctx) {
			writeJsonBytes(jsonBytes, ctx)
		}
	}
}

type GetListDataResult struct {
	Client    string
	Database  string
	Table     string
	Clients   []string
	Databases []string
	Tables    []string
}

func getListData(ctx *fasthttp.RequestCtx) {
	r := &GetListDataResult{
		Clients:   make([]string, 0),
		Databases: make([]string, 0),
		Tables:    make([]string, 0),
	}

	r.Client = xbytes.BytesToStr(ctx.FormValue("client"))
	r.Database = xbytes.BytesToStr(ctx.FormValue("db"))

	clientRS, err := logClientService.GetClients(context.Background(), &logs.LogClientsQuery{})
	if handleErr(err, ctx) {
		return
	}
	r.Clients = clientRS.LogClients

	if r.Client != "" {
		// Has client id provided
		dbRS, err := logClientService.GetDatabases(context.Background(), &logs.DatabasesQuery{
			ClientID: r.Client,
		})
		if handleErr(err, ctx) {
			return
		}
		r.Databases = dbRS.Databases

		if r.Database != "" {
			tabRS, err := logClientService.GetTables(context.Background(), &logs.TablesQuery{
				Database: r.Database,
			})
			if handleErr(err, ctx) {
				return
			}
			r.Tables = tabRS.Tables
		}
	}

	jsonBytes, err := json.Marshal(r)
	if !handleErr(err, ctx) {
		writeJsonBytes(jsonBytes, ctx)
	}
}

func handleErr(err error, ctx *fasthttp.RequestCtx) bool {
	if err != nil {
		ctx.SetStatusCode(400)
		return true
	}
	return false
}

func writeJsonBytes(jsonBytes []byte, ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set(xhttp.HEADER_CTYPE, xhttp.CTYPE_JSON)
	ctx.Write(jsonBytes)
}

func serveEmbedFiles(router *router.Router, webPath, physiblePath string, emd embed.FS) {
	if !strings.HasSuffix(webPath, _suffix) {
		panic("path must end with " + _suffix + " in path '" + webPath + "'")
	}

	router.GET(webPath, func(ctx *fasthttp.RequestCtx) {
		filepath := ctx.UserValue(_filepath).(string)
		if filepath == "" {
			filepath = "index.html"
		}

		filepath = physiblePath + "/" + filepath

		file, err := emd.Open(filepath) // embed file doesn't need to close
		if err == nil {
			ext := fp.Ext(filepath)
			cType := mime.TypeByExtension(ext)

			if cType != "" {
				ctx.SetContentType(cType)
			}
			ctx.Response.SetBodyStream(file, -1)
			return
		}

		ctx.SetStatusCode(404)
		ctx.WriteString("NOT FOUND")
	})
}

func registerConsulServiceInfo(cp xconfig.IConfigProvider) error {
	// Read configuration
	consulAddr := cp.GetString("Consul.Addr")
	consulToken := cp.GetString("Consul.Token")
	serviceName := cp.GetString("Consul.Service.Name")
	serviceCheckTimeout := cp.GetString("Consul.Service.Check.Timeout")
	serviceCheckInterval := cp.GetString("Consul.Service.Check.Interval")
	serviceHost := cp.GetString("Consul.Service.Host")
	servicePort := cp.GetInt("Consul.Service.Port")
	serviceID := fmt.Sprintf("%v:%v", serviceHost, servicePort)

	// Service center client
	consulConfig := api.DefaultConfig()
	consulConfig.Address = consulAddr
	consulConfig.Token = consulToken
	consulClient, err := api.NewClient(consulConfig)
	if err != nil {
		return xerr.WithStack(err)
	}
	consulAgent := consulClient.Agent()

	// Register service in service center
	err = consulAgent.ServiceRegister(&api.AgentServiceRegistration{
		ID:   serviceID,   // Service node name
		Name: serviceName, // Service name
		// Tags:    r.Tag,                                        // Tags, can be empty
		Address: serviceHost, // Service IP
		Port:    servicePort, // Service port
		Check: &api.AgentServiceCheck{ // Health check
			Interval:                       serviceCheckInterval, // Health check interval
			TCP:                            fmt.Sprintf("%s:%d", serviceHost, servicePort),
			DeregisterCriticalServiceAfter: serviceCheckTimeout, // Deregistration time, equivalent to expiration time
		},
	})

	return xerr.WithStack(err)
}
