package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func createKey(id int64) string {
	return fmt.Sprintf("product:%d", id)
}

func (r *RedisCache) SetProduct(ctx context.Context, product *domain.Product) error {
	key := createKey(product.Id)
	data, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("%w: error marshalling product: %s", domain.ErrInternalCache, err.Error())
	}
	err = r.client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("%w: failed to store product to cache: %s", domain.ErrInternalCache, err.Error())
	}
	return nil
}

func (r *RedisCache) GetJSONProductById(ctx context.Context, id int64) ([]byte, error) {
	key := createKey(id)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w: failed to find product %d in cache", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: failed to get product %d from cache: %s", domain.ErrInternalCache, id, err.Error())
	}
	return data, nil
}

func (r *RedisCache) DeleteProductById(ctx context.Context, id int64) error {
	key := createKey(id)
	result, err := r.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("%w: failed to delete product %d from cache: %s", domain.ErrInternalCache, id, err)
	}
	if result == 0 {
		return fmt.Errorf("%w: product with id=%d not found in cache", domain.ErrNotFound, id)
	}
	return nil
}

func (c *RedisCache) ClearCache(ctx context.Context) error {
	_, err := c.client.FlushDB(ctx).Result()
	if err != nil {
		return fmt.Errorf("%w: failed to clear cache: %s", domain.ErrInternalCache, err.Error())
	}
	return nil
}
