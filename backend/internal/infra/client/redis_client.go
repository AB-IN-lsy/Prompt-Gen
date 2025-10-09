/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 16:34:40
 * @FilePath: \electron-go-app\backend\internal\infra\redis_client.go
 * @LastEditTime: 2025-10-09 16:34:47
 */
package infra

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"electron-go-app/backend/internal/config"

	"github.com/redis/go-redis/v9"
)

const (
	envRedisEndpoint = "REDIS_ENDPOINT"
	envRedisPassword = "REDIS_PASSWORD"
	envRedisDB       = "REDIS_DB"
)

const (
	defaultRedisPort = 6379
	defaultRedisDB   = 0
	defaultRedisTTL  = 5 * time.Second
)

// RedisOptions 描述连接 Redis 所需的配置。
type RedisOptions struct {
	Host     string
	Port     int
	Password string
	DB       int
	Timeout  time.Duration
}

// NewDefaultRedisOptions 从环境变量读取 Redis 连接信息。
func NewDefaultRedisOptions() (RedisOptions, error) {
	config.LoadEnvFiles()

	endpoint := strings.TrimSpace(os.Getenv(envRedisEndpoint))
	if endpoint == "" {
		return RedisOptions{}, fmt.Errorf("%s not set", envRedisEndpoint)
	}

	host, port, err := parseEndpointWithDefault(endpoint, defaultRedisPort)
	if err != nil {
		return RedisOptions{}, fmt.Errorf("invalid redis endpoint: %w", err)
	}

	db := defaultRedisDB
	if rawDB := strings.TrimSpace(os.Getenv(envRedisDB)); rawDB != "" {
		value, err := strconv.Atoi(rawDB)
		if err != nil {
			return RedisOptions{}, fmt.Errorf("invalid redis db: %w", err)
		}
		db = value
	}

	password := os.Getenv(envRedisPassword)

	return RedisOptions{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
		Timeout:  defaultRedisTTL,
	}, nil
}

// NewRedisClient 根据配置创建 redis.Client，并执行一次 PING 验证连接。
func NewRedisClient(opts RedisOptions) (*redis.Client, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("redis host is required")
	}
	if opts.Port == 0 {
		opts.Port = defaultRedisPort
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultRedisTTL
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Password: opts.Password,
		DB:       opts.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

func parseEndpointWithDefault(endpoint string, defaultPort int) (string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", 0, fmt.Errorf("endpoint is empty")
	}

	if !strings.Contains(endpoint, ":") {
		return endpoint, defaultPort, nil
	}

	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
