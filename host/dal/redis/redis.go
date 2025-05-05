package redis

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/DreamvatLab/go/xbytes"
	"github.com/DreamvatLab/go/xconfig"
	"github.com/DreamvatLab/go/xerr"
	"github.com/DreamvatLab/go/xlog"
	"github.com/DreamvatLab/go/xredis"
	"github.com/DreamvatLab/logs"
	goredis "github.com/redis/go-redis/v9"
)

const (
	_KEY = "account:Logs"
)

var (
	_clientsMap  map[string]*logs.LogClient
	_cacheLocker = new(sync.RWMutex)
)

type RedisDAL struct {
	client goredis.UniversalClient
}

func (o *RedisDAL) Init(cp xconfig.IConfigProvider) {
	connStr := cp.GetString("ConnectionStrings.Redis")

	redisConfig, err := xredis.ParseRedisConfig(connStr)
	xerr.FatalIfErr(err)

	o.client = xredis.NewClient(redisConfig)

	err = o.refreshCache()
	xerr.FatalIfErr(err)

	go o.monitor()
}

func (o *RedisDAL) monitor() {
	// Subscribe key changes
	sub := o.client.Subscribe(context.Background(), "__keyspace@0__:"+_KEY)

	// refresh cache when there's a change
	for {
		msg := <-sub.Channel()
		o.refreshCache()
		xlog.Debugf("Cache refreshed (Channel:'%s', Pattern:'%s', Payload:'%s', PayloadSlice:%v)", msg.Channel, msg.Pattern, msg.Payload, msg.PayloadSlice)
	}
}
func (o *RedisDAL) refreshCache() error {
	_cacheLocker.Lock()
	defer _cacheLocker.Unlock()

	var clients []*logs.LogClient
	clients, err := o.GetClients(nil)
	if err != nil {
		return xerr.WithStack(err)
	}

	_clientsMap = make(map[string]*logs.LogClient, len(clients))
	for _, x := range clients {
		_clientsMap[x.ID] = x
	}

	return nil
}

func (o *RedisDAL) InsertClient(in *logs.LogClient) error {
	return o.UpdateClient(in)
}
func (o *RedisDAL) GetClient(id string) (r *logs.LogClient, err error) {
	var ok bool
	_cacheLocker.RLock()
	r, ok = _clientsMap[id]
	_cacheLocker.RUnlock()
	if ok {
		return r, nil
	}

	r = new(logs.LogClient)
	rs, err := o.client.HGet(context.Background(), _KEY, id).Result()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}

		return nil, xerr.WithStack(err)
	}

	err = json.Unmarshal(xbytes.StrToBytes(rs), &r)
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	_cacheLocker.Lock()
	_clientsMap[id] = r
	_cacheLocker.Unlock()

	return r, nil
}
func (o *RedisDAL) UpdateClient(in *logs.LogClient) error {
	jsonStr, err := json.Marshal(in)
	if err != nil {
		return xerr.WithStack(err)
	}
	_, err = o.client.HSet(context.Background(), _KEY, in.ID, jsonStr).Result()
	if err != nil {
		return xerr.WithStack(err)
	}

	_cacheLocker.Lock()
	defer _cacheLocker.Unlock()
	_clientsMap[in.ID] = in

	return nil
}
func (o *RedisDAL) DeleteClient(id string) error {
	_, err := o.client.HDel(context.Background(), _KEY, id).Result()
	if err != nil {
		return xerr.WithStack(err)
	}

	_cacheLocker.Lock()
	defer _cacheLocker.Unlock()
	delete(_clientsMap, id)

	return nil
}
func (o *RedisDAL) GetClients(*logs.LogClientsQuery) ([]*logs.LogClient, error) {
	rs, err := o.client.HGetAll(context.Background(), _KEY).Result()
	if err != nil {
		return nil, xerr.WithStack(err)
	}

	r := make([]*logs.LogClient, 0, len(rs))
	for _, x := range rs {
		var client *logs.LogClient
		err = json.Unmarshal(xbytes.StrToBytes(x), &client)
		if err != nil {
			return nil, xerr.WithStack(err)
		}
		r = append(r, client)
	}

	// Sort
	sort.Slice(r, func(i, j int) bool {
		return r[i].ID < r[j].ID
	})

	return r, nil
}
