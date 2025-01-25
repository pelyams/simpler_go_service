package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/pelyams/simpler_go_service/internal/domain"
	"github.com/pelyams/simpler_go_service/testhelpers"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ProductCacheTestSuite struct {
	suite.Suite
	cacheContainer *testhelpers.RedisContainer
	cache          *RedisCache
	ctx            context.Context
}

func (suite *ProductCacheTestSuite) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *ProductCacheTestSuite) SetupTest() {
	t := suite.T()
	redisContainer, err := testhelpers.CreateRedisContainer(suite.ctx)
	if err != nil {
		t.Fatal("failed to create RedisContainer: ", err)
	}
	suite.cacheContainer = redisContainer

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisContainer.ConnectionString,
		DB:   0,
	})
	redisClient.ConfigSet(context.Background(), "maxmemory", "10mb")
	redisClient.ConfigSet(context.Background(), "maxmemory-policy", "allkeys-lru")

	cache := NewRedisCache(redisClient)
	if err != nil {
		t.Fatal("failed to create RedisCache: ", err)
	}
	suite.cache = cache
}

func (suite *ProductCacheTestSuite) TearDownTest() {
	if err := suite.cacheContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatal("error terminating postgres container: ", err)
	}
}

func TestCustomerRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ProductCacheTestSuite))
}

func (suite *ProductCacheTestSuite) TestSetProduct() {
	t := suite.T()

	testId := 515
	testProduct := domain.Product{
		Id:             int64(testId),
		Name:           "Product for testing store operation",
		AdditionalInfo: "This product help us to indicate if store operation works as intended",
	}

	err := suite.cache.SetProduct(suite.ctx, &testProduct)
	assert.NoError(t, err)

	product, err := suite.cache.client.Get(suite.ctx, fmt.Sprintf("product:%d", testId)).Result()
	if err != nil {
		t.Fatal("failed to retrieve product: ", err)
	}
	var retrievedProduct domain.Product
	err = json.Unmarshal([]byte(product), &retrievedProduct)
	if err != nil {
		t.Fatal("failed to unmarshall test product json: ", err)
	}
	assert.Equal(t, testProduct, retrievedProduct)

	//case disconnected
	err = suite.cacheContainer.Stop(suite.ctx, nil)
	if err != nil {
		t.Fatal("failed to stop redis container: ", err)
	}
	err = suite.cache.SetProduct(suite.ctx, &testProduct)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalCache))

}

func (suite *ProductCacheTestSuite) TestGetJSONProductById() {
	testCases := []struct {
		name        string
		setProduct  bool
		testId      int64
		testProduct *domain.Product
		expectedErr error
	}{
		{
			name:       "get json product from cache - success",
			setProduct: true,
			testId:     45,
			testProduct: &domain.Product{
				Id:             45,
				Name:           "Product for testing delete operation",
				AdditionalInfo: "This product help us to indicate if delete operation works as intended",
			},
			expectedErr: nil,
		},
		{
			name:        "get json product from cache - no found",
			testId:      48,
			expectedErr: domain.ErrNotFound,
		},
		{
			name:        "get json product from cache - cache disconnected",
			testId:      49,
			expectedErr: domain.ErrInternalCache,
		},
	}
	t := suite.T()
	for _, tt := range testCases {
		suite.Run(tt.name, func() {
			key := fmt.Sprintf("product:%d", tt.testId)
			data, err := json.Marshal(tt.testProduct)
			if err != nil {
				t.Fatal("failed to marshall test product: ", err)
			}
			if tt.setProduct {
				err = suite.cache.client.Set(suite.ctx, key, data, 0).Err()
				if err != nil {
					t.Fatal("failed to set test product: ", err)
				}
			}
			if tt.name == "get json product from cache - cache disconnected" {
				err = suite.cacheContainer.Stop(suite.ctx, nil)
				if err != nil {
					t.Fatal("failed to stop redis container: ", err)
				}
			}
			productAsBytes, err := suite.cache.GetJSONProductById(suite.ctx, tt.testId)
			if tt.expectedErr == nil {
				assert.NoError(t, err)
				assert.NotNil(t, productAsBytes)
				assert.Equal(t, data, productAsBytes)
			} else {
				assert.Nil(t, productAsBytes)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			}
		})
	}
}

func (suite *ProductCacheTestSuite) TestDeleteProductById() {
	testcases := []struct {
		name        string
		testId      int64
		setProduct  bool
		testProduct domain.Product
		expectedErr error
	}{
		{
			name:       "delete product from cache - success",
			testId:     122,
			setProduct: true,
			testProduct: domain.Product{
				Id:             122,
				Name:           "Product for testing delete operation",
				AdditionalInfo: "This product help us to indicate if delete operation works as intended",
			},
			expectedErr: nil,
		},
		{
			name:        "delete product from cache - not found",
			testId:      125,
			expectedErr: domain.ErrNotFound,
		},
		{
			name:        "delete product from cache - cache disconnected",
			testId:      129,
			expectedErr: domain.ErrInternalCache,
		},
	}
	t := suite.T()
	for _, tt := range testcases {
		suite.Run(tt.name, func() {
			key := fmt.Sprintf("product:%d", tt.testId)
			if tt.setProduct {
				data, err := json.Marshal(tt.testProduct)
				if err != nil {
					t.Fatal("failed to marshall test product", err)
				}
				err = suite.cache.client.Set(suite.ctx, key, data, 0).Err()
				if err != nil {
					t.Fatal("failed to set test product: ", err)
				}
			}
			if tt.name == "delete product from cache - cache disconnected" {
				err := suite.cacheContainer.Stop(suite.ctx, nil)
				if err != nil {
					t.Fatal("failed to stop redis container: ", err)
				}
			}
			err := suite.cache.DeleteProductById(suite.ctx, tt.testId)
			if tt.expectedErr == nil {
				assert.NoError(t, err)
				exists, err := suite.cache.client.Exists(suite.ctx, key).Result()
				if err != nil {
					t.Fatal("failed to check product existence: ", err)
				}
				assert.Zero(t, exists)
			} else {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			}
		})
	}
}

func (suite *ProductCacheTestSuite) TestClearCache() {
	t := suite.T()

	garbageDataLen := 100
	var keysAndProducts []interface{}
	for i := range garbageDataLen {
		key := fmt.Sprintf("product:%d", i)
		product := domain.Product{
			Id:             int64(i),
			Name:           fmt.Sprintf("Product #%d", i),
			AdditionalInfo: fmt.Sprintf("Product #%d description", i),
		}
		data, err := json.Marshal(product)
		if err != nil {
			t.Fatal("failed to marshall test product: ", err)
		}
		keysAndProducts = append(keysAndProducts, key, data)
	}
	err := suite.cache.client.MSet(suite.ctx, keysAndProducts).Err()
	if err != nil {
		t.Fatal("failed to mset test data: ", err)
	}

	err = suite.cache.ClearCache(suite.ctx)
	assert.NoError(t, err)

	keysLeft := suite.cache.client.DBSize(suite.ctx).Val()
	assert.Equal(t, int64(0), keysLeft)

	//case disconnected
	err = suite.cacheContainer.Stop(suite.ctx, nil)
	if err != nil {
		t.Fatal("failed to stop redis container: ", err)
	}
	err = suite.cache.ClearCache(suite.ctx)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalCache))
}
