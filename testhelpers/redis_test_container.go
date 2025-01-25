package testhelpers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type RedisContainer struct {
	*redis.RedisContainer
	ConnectionString string
}

func CreateRedisContainer(ctx context.Context) (*RedisContainer, error) {
	redisContainer, err := redis.Run(ctx,
		"redis:7.2",
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
		testcontainers.WithHostPortAccess(6379),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").WithStartupTimeout(3*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis container: %w", err)
	}
	host, err := redisContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get redis host: %w", err)
	}

	mappedPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get redis mapped port: %w", err)
	}
	connectionString := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	return &RedisContainer{
		RedisContainer:   redisContainer,
		ConnectionString: connectionString,
	}, nil
}
