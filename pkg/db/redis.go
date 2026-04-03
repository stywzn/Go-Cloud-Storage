package db

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var RDB *redis.Client

func InitRedis(addr, password string, db int) {
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 测试连接
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		panic("Failed to connect to Redis: " + err.Error())
	}
}
