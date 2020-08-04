package redis

import (
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis"

	"keyayun.com/seal-micro-runner/pkg/config"
	"keyayun.com/seal-micro-runner/pkg/logger"
	pb "keyayun.com/seal-micro-runner/pkg/proto"
)

var tableIndex = struct {
	Instances int
}{
	14,
}

const instancesKey = "Instances"

var (
	client *redis.Client
	log    = logger.WithNamespace("redis")
	conf   = config.Config
)

// init 初始化Redis
func init() {
	client = redis.NewClient(&redis.Options{
		Addr:         conf.GetString("redis.addr"),
		Password:     conf.GetString("redis.password"),
		DB:           0,
		PoolSize:     conf.GetInt("redis.poolSize"),
		MinIdleConns: 1,
	})

	_, err := client.Ping().Result()
	if err != nil {
		panic(fmt.Errorf("Fatal error redis: %s", err))
	}
}

// RegisterInstance 注册instance
func RegisterInstance(serviceName string, s *pb.TokenModel) error {
	b, err := json.Marshal(s)
	if err != nil {
		log.Errorf("RegisterInstance failed: %v", err)
		return err
	}
	pl := client.TxPipeline()
	pl.Select(tableIndex.Instances)
	pl.HMSet(instancesKey, map[string]interface{}{
		serviceName: b,
	})
	_, err = pl.Exec()
	return err
}

// UnregisterInstance 反注册instance
func UnregisterInstance(serviceName string, s *pb.TokenModel) error {
	pl := client.TxPipeline()
	pl.Select(tableIndex.Instances)
	pl.HDel(instancesKey, serviceName+"_"+s.Domain)
	_, err := pl.Exec()
	return err
}

// GetInstance 获取已注册的instance
func GetInstance(tokenID string) (*pb.TokenModel, error) {
	pl := client.TxPipeline()
	pl.Select(tableIndex.Instances)
	get := pl.HGet(instancesKey, tokenID)
	_, err := pl.Exec()
	if err != nil {
		log.Errorf("GetInstance failed when exec redis command: %v", err)
		return nil, err
	}
	js, err := get.Result()
	if err != nil {
		log.Errorf("GetInstance failed when get redis result: %v", err)
		return nil, err
	}
	var st pb.TokenModel
	err = json.Unmarshal([]byte(js), &st)
	if err != nil {
		log.Errorf("GetInstance failed when unmarshal json: %v", err)
		return nil, err
	}
	return &st, nil
}

// GetRunnerInstances func
func GetRunnerInstances() (map[string]*pb.TokenModel, error) {
	pl := client.TxPipeline()
	pl.Select(tableIndex.Instances)
	get := pl.HGetAll(instancesKey)
	_, err := pl.Exec()
	if err != nil {
		log.Errorf("GetRunnerInstances failed when exec redis command: %v", err)
		return nil, err
	}
	jss, err := get.Result()
	if err != nil {
		log.Errorf("GetRunnerInstances failed when get redis result: %v", err)
		return nil, err
	}
	var sts map[string]*pb.TokenModel
	for serviceName, v := range jss {
		var st pb.TokenModel
		err = json.Unmarshal([]byte(v), &st)
		if err != nil {
			log.Errorf("GetRunnerInstances failed when unmarshal json: %v", err)
			return nil, err
		}
		sts[serviceName] = &st
	}
	return sts, nil
}
