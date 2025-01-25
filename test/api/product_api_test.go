package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/pelyams/simpler_go_service/testhelpers"

	"github.com/pelyams/simpler_go_service/internal/adapters/cache"
	"github.com/pelyams/simpler_go_service/internal/adapters/repository"
	"github.com/pelyams/simpler_go_service/internal/domain"
	"github.com/pelyams/simpler_go_service/internal/routing"
	"github.com/pelyams/simpler_go_service/internal/service"
)

type TestSuite struct {
	suite.Suite
	pgContainer         *testhelpers.PostgresContainer
	cacheContainer      *testhelpers.RedisContainer
	cache               *redis.Client
	db                  *sql.DB
	server              *httptest.Server
	client              *http.Client
	ctx                 context.Context
	pgContainerAlive    bool
	cacheContainerAlive bool
}

func (suite *TestSuite) SetupSuite() {
	suite.ctx = context.Background()

}

func (suite *TestSuite) SetupTest() {
	redisContainer, err := testhelpers.CreateRedisContainer(suite.ctx)
	if err != nil {
		log.Fatal(err)
	}
	suite.cacheContainer = redisContainer

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisContainer.ConnectionString,
		DB:   0,
	})
	redisClient.ConfigSet(context.Background(), "maxmemory", "10mb")
	redisClient.ConfigSet(context.Background(), "maxmemory-policy", "allkeys-lru")

	suite.cache = redisClient

	pgContainer, err := testhelpers.CreatePostgresContainer(suite.ctx)
	if err != nil {
		log.Fatal(err)
	}
	suite.pgContainer = pgContainer

	databaseClient, err := sql.Open("postgres", pgContainer.ConnectionString)
	if err != nil {
		log.Fatal(err)
	}

	suite.db = databaseClient

	repo := repository.NewPostgresRepository(databaseClient)
	cache := cache.NewRedisCache(redisClient)
	service := service.NewResourceService(repo, cache)

	handler := routing.NewProductHandler(service)
	router := routing.NewRouter(handler).SetupRoutes()

	logger, err := routing.NewLogger(0, "test_log.log")
	suite.Require().NoError(err)

	suite.server = httptest.NewServer(logger.LoggerMiddleware(router))
	suite.client = &http.Client{Timeout: 5 * time.Second}
	suite.pgContainerAlive = true
	suite.cacheContainerAlive = true
}

func (s *TestSuite) TearDownTest() {
	s.pgContainer.Terminate(s.ctx)
	s.cacheContainer.Terminate(s.ctx)
	s.server.Close()
}

func (s *TestSuite) SetupSubTest() {
	if s.pgContainerAlive {
		_, err := s.db.Exec("TRUNCATE TABLE products")
		if err != nil {
			s.T().Fatal(err)
		}
	}
	if s.cacheContainerAlive {
		_, err := s.cache.FlushDB(s.ctx).Result()
		if err != nil {
			s.T().Fatal(err)
		}
	}

}

func (s *TestSuite) makeRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, s.server.URL+path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.client.Do(req)
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) TestGetProductById() {
	tests := []struct {
		name           string
		setupProduct   bool
		testProduct    map[string]string
		productId      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:         "get product - success",
			setupProduct: true,
			testProduct: map[string]string{
				"name":           "Test product name",
				"additionalInfo": "Some additional info for test product",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get product - product does not exist",
			productId:      "666",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Product not found",
		},
		{
			name:           "get product - invalid product id: invalid type",
			productId:      "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid product id",
		},
		{
			name:           "get product - invalid product id: invalid value",
			productId:      "-4",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid product id",
		},
		{
			name:         "get product (cache disconnected) - success",
			setupProduct: true,
			testProduct: map[string]string{
				"name":           "Test product name",
				"additionalInfo": "Some additional info for test product",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "get product - db disconnected",
			productId: "13",

			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			var productId string
			if tt.setupProduct {
				var id int64
				err := s.db.QueryRow("INSERT INTO products (name, additional_info) VALUES ($1, $2) RETURNING id", tt.testProduct["name"], tt.testProduct["additionalInfo"]).Scan(&id)
				require.NoError(s.T(), err)
				productId = strconv.Itoa(int(id))
			} else {
				productId = tt.productId
			}

			if tt.name == "get product (cache disconnected) - success" {
				err := s.cacheContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.cacheContainerAlive = false
			}

			if tt.name == "get product - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}

			resp, err := s.makeRequest("GET", "/product/"+productId, nil)
			defer resp.Body.Close()

			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			decoder.UseNumber()
			err = decoder.Decode(&response)
			require.NoError(s.T(), err)

			if tt.expectedError != "" {
				assert.Equal(s.T(), tt.expectedError, response["error"])
			} else {
				assert.NotEmpty(s.T(), response["name"])
				assert.Equal(s.T(), tt.testProduct["name"], response["name"])
				assert.NotEmpty(s.T(), response["additionalInfo"])
				assert.Equal(s.T(), tt.testProduct["additionalInfo"], response["additionalInfo"])
				id, _ := strconv.ParseInt(productId, 10, 64)
				retrievedId, err := response["id"].(json.Number).Int64()
				require.NoError(s.T(), err)
				assert.Equal(s.T(), id, retrievedId)

				if tt.name == "get product - success" {
					cachedProduct, err := s.cache.Get(s.ctx, "product:"+productId).Bytes()
					require.NoError(s.T(), err)
					var cached map[string]interface{}
					json.Unmarshal(cachedProduct, &cached)
					assert.Equal(s.T(), tt.testProduct["name"], cached["name"])
					assert.Equal(s.T(), tt.testProduct["additionalInfo"], cached["additionalInfo"])
				}

			}
		})
	}
}

func (s *TestSuite) TestUpdateProduct() {
	tests := []struct {
		name           string
		setupProduct   bool
		oldProduct     map[string]interface{}
		productId      string
		updatedProduct map[string]string
		expectedStatus int
		expectedError  string
	}{
		{
			name:         "update product - success",
			setupProduct: true,
			oldProduct: map[string]interface{}{
				"id":             int64(419),
				"name":           "Older product name",
				"additionalInfo": "Some additional info for older product",
			},
			productId: "419",
			updatedProduct: map[string]string{
				"name":           "Renewed product name",
				"additionalInfo": "Some additional info for renewed product",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "update product - invalid id",
			productId: "invalid-id",
			updatedProduct: map[string]string{
				"name":           "Renewed product name",
				"additionalInfo": "Some additional info for renewed product",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid product id",
		},
		{
			name:      "update product - invalid body",
			productId: "71",
			updatedProduct: map[string]string{
				"invalidFieldOne":   "...",
				"invalidFieldTwo":   "...",
				"invalidFieldThree": "..",
			},

			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:      "update product - no product with specified ID",
			productId: "2007",
			updatedProduct: map[string]string{
				"name":           "Renewed product name",
				"additionalInfo": "Some additional info for renewed product",
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "Product not found",
		},
		{
			name:      "update product - db disconnected",
			productId: "2013",
			updatedProduct: map[string]string{
				"name":           "Renewed product name",
				"additionalInfo": "Some additional info for renewed product",
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
		{
			name:      "update product - cache disconnected",
			productId: "2013",
			updatedProduct: map[string]string{
				"name":           "Renewed product name",
				"additionalInfo": "Some additional info for renewed product",
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupProduct {
				res := s.db.QueryRow("INSERT INTO products (id, name, additional_info) VALUES ($1, $2, $3)", tt.oldProduct["id"], tt.oldProduct["name"], tt.oldProduct["additionalInfo"])
				require.NoError(s.T(), res.Err())
			}
			if tt.name == "update product - cache disconnected" {
				err := s.cacheContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.cacheContainerAlive = false
			}

			if tt.name == "update product - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}

			resp, err := s.makeRequest("PUT", "/product/"+tt.productId, tt.updatedProduct)
			defer resp.Body.Close()

			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			decoder.UseNumber()
			err = decoder.Decode(&response)
			assert.NoError(s.T(), err)

			if tt.expectedError != "" {
				assert.Equal(s.T(), tt.expectedError, response["error"])
			} else {
				assert.NotEmpty(s.T(), response["name"])
				assert.Equal(s.T(), tt.oldProduct["name"], response["name"])
				assert.NotEmpty(s.T(), response["additionalInfo"])
				assert.Equal(s.T(), tt.oldProduct["additionalInfo"], response["additionalInfo"])
				id, err := strconv.ParseInt(tt.productId, 10, 64)
				require.NoError(s.T(), err)
				retrievedId, err := response["id"].(json.Number).Int64()
				require.NoError(s.T(), err)
				assert.Equal(s.T(), id, retrievedId)

				var updatedProduct domain.Product
				err = s.db.QueryRow("SELECT id, name, additional_info FROM products WHERE id=$1", tt.productId).
					Scan(&updatedProduct.Id, &updatedProduct.Name, &updatedProduct.AdditionalInfo)
				require.NoError(s.T(), err)
				assert.Equal(s.T(), id, updatedProduct.Id)
				assert.Equal(s.T(), tt.updatedProduct["name"], updatedProduct.Name)
				assert.Equal(s.T(), tt.updatedProduct["additionalInfo"], updatedProduct.AdditionalInfo)

				exists, err := s.cache.Exists(s.ctx, "product:"+tt.productId).Result()
				require.NoError(s.T(), err)
				assert.Zero(s.T(), exists)
			}

		})
	}
}

func (s *TestSuite) TestDeleteProduct() {
	tests := []struct {
		name             string
		setupProduct     bool
		productForDelete map[string]interface{}
		productId        string
		expectedStatus   int
		expectedError    string
	}{
		{
			name:         "delete product - success",
			setupProduct: true,
			productForDelete: map[string]interface{}{
				"id":             int64(421),
				"name":           "Deleted product name",
				"additionalInfo": "Some additional info for deleted product",
			},
			productId:      "421",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete product - invalid ID",
			productId:      "invalid-id",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid product id",
		},
		{
			name:           "delete product - no product with specified ID",
			productId:      "2007",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Product not found",
		},
		{
			name:           "delete product - db disconnected",
			productId:      "2013",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
		{
			name:           "delete product - cache disconnected",
			productId:      "2013",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.setupProduct {
				res := s.db.QueryRow("INSERT INTO products (id, name, additional_info) VALUES ($1, $2, $3)", tt.productForDelete["id"], tt.productForDelete["name"], tt.productForDelete["additionalInfo"])
				require.NoError(s.T(), res.Err())
			}
			if tt.name == "delete product - cache disconnected" {
				err := s.cacheContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.cacheContainerAlive = false
			}

			if tt.name == "delete product - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}
			resp, err := s.makeRequest("DELETE", "/product/"+tt.productId, nil)
			defer resp.Body.Close()

			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			decoder.UseNumber()
			err = decoder.Decode(&response)
			assert.NoError(s.T(), err)

			if tt.expectedError != "" {
				assert.Equal(s.T(), tt.expectedError, response["error"])
			} else {
				assert.NotEmpty(s.T(), response["name"])
				assert.Equal(s.T(), tt.productForDelete["name"], response["name"])
				assert.NotEmpty(s.T(), response["additionalInfo"])
				assert.Equal(s.T(), tt.productForDelete["additionalInfo"], response["additionalInfo"])
				id, err := strconv.ParseInt(tt.productId, 10, 64)
				require.NoError(s.T(), err)
				retrievedId, err := response["id"].(json.Number).Int64()
				require.NoError(s.T(), err)
				assert.Equal(s.T(), id, retrievedId)

				queryErr := s.db.QueryRow("SELECT id, name, additional_info FROM products WHERE id=$1", tt.productId).Scan()

				assert.Error(s.T(), queryErr)
				assert.True(s.T(), errors.Is(queryErr, sql.ErrNoRows))

				exists, err := s.cache.Exists(s.ctx, "product:"+tt.productId).Result()
				require.NoError(s.T(), err)
				assert.Zero(s.T(), exists)
			}

		})
	}
}

func (s *TestSuite) TestCreateUser() {
	tests := []struct {
		name           string
		newProduct     map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "create product - valid product creation",
			newProduct: map[string]interface{}{
				"name":           "Product #1",
				"additionalInfo": "Product #1 description",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create product - missing additional info",
			newProduct: map[string]interface{}{
				"name": "Product #2",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "create product - missing additional name",
			newProduct: map[string]interface{}{
				"additionalInfo": "Product #3 description",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "create product - invalid type name",
			newProduct: map[string]interface{}{
				"name":           4,
				"additionalInfo": "Valid info",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		//weird scenario by weird design:
		{
			name: "create product - cache disconnected",
			newProduct: map[string]interface{}{
				"name":           "Product #13",
				"additionalInfo": "Product #13 description",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "create product - db disconnected",
			newProduct: map[string]interface{}{
				"name":           "Product #13",
				"additionalInfo": "Product #13 description",
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if tt.name == "create product - cache disconnected" {
				err := s.cacheContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.cacheContainerAlive = false
			}

			if tt.name == "create product - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}

			resp, err := s.makeRequest("POST", "/product/", tt.newProduct)
			defer resp.Body.Close()

			require.NoError(s.T(), err)
			assert.Equal(s.T(), tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			decoder.UseNumber()
			err = decoder.Decode(&response)
			assert.NoError(s.T(), err)

			if tt.expectedError != "" {
				assert.Equal(s.T(), tt.expectedError, response["error"])
			} else {
				require.NotEmpty(s.T(), response["id"])
				retrievedId, err := response["id"].(json.Number).Int64()
				require.NoError(s.T(), err)
				assert.Positive(s.T(), retrievedId)

				var storedProduct domain.Product
				queryErr := s.db.QueryRow("SELECT id, name, additional_info FROM products WHERE id=$1", retrievedId).
					Scan(&storedProduct.Id, &storedProduct.Name, &storedProduct.AdditionalInfo)
				assert.NoError(s.T(), queryErr)
				assert.Equal(s.T(), retrievedId, storedProduct.Id)
				assert.Equal(s.T(), tt.newProduct["name"], storedProduct.Name)
				assert.Equal(s.T(), tt.newProduct["additionalInfo"], storedProduct.AdditionalInfo)

				if tt.name == "create product - valid product creation" {
					cachedProduct, err := s.cache.Get(s.ctx, fmt.Sprintf("product:%d", retrievedId)).Bytes()
					require.NoError(s.T(), err)
					var cached map[string]interface{}
					json.Unmarshal(cachedProduct, &cached)
					assert.Equal(s.T(), tt.newProduct["name"], cached["name"])
					assert.Equal(s.T(), tt.newProduct["additionalInfo"], cached["additionalInfo"])
				}
			}

		})
	}
}

func (s *TestSuite) TestDeleteAll() {
	testCases := []struct {
		name string

		setupProducts  bool
		garbageData    []domain.Product
		expectedResult int64
		expectedStatus int
		expectedError  string
	}{
		{
			name:          "delete all products - success",
			setupProducts: true,
			garbageData: []domain.Product{
				domain.Product{
					Id:             1,
					Name:           "Test product #1",
					AdditionalInfo: "Test product #1 info",
				},
				domain.Product{
					Id:             3,
					Name:           "Test product #3",
					AdditionalInfo: "Test product #3 info",
				},
				domain.Product{
					Id:             7,
					Name:           "Test product #7",
					AdditionalInfo: "Test product #7 info",
				},
			},
			expectedResult: 3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete all products - db disconnected",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
		{
			name:           "delete all products - cache disconnected",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			if tt.setupProducts {
				query := "INSERT INTO products (id, name, additional_info) VALUES "
				dataToInsert := make([]string, len(tt.garbageData))
				for i, data := range tt.garbageData {
					dataToInsert[i] = fmt.Sprintf("(%d, '%s', '%s')", data.Id, data.Name, data.AdditionalInfo)
				}
				query = fmt.Sprintf("%s %s", query, strings.Join(dataToInsert, ", "))
				_, err := s.db.Query(query)
				s.Require().NoError(err)
			}
			if tt.name == "delete all products - cache disconnected" {
				err := s.cacheContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.cacheContainerAlive = false
			}

			if tt.name == "delete all products - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}
			resp, err := s.makeRequest("DELETE", "/products", nil)
			defer resp.Body.Close()

			s.Assert().Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			decoder.UseNumber()
			err = decoder.Decode(&response)
			require.NoError(s.T(), err)

			if tt.expectedStatus == http.StatusOK {
				s.Require().NoError(err)
				s.Assert().NotNil(resp)

				deletedRows, err := response["deletedRows"].(json.Number).Int64()
				s.Require().NoError(err)
				s.Assert().Equal(tt.expectedResult, deletedRows)

				var dbEntryCount int64
				err = s.db.QueryRow("SELECT COUNT (*) FROM products").Scan(&dbEntryCount)
				s.Require().NoError(err)
				s.Assert().Zero(dbEntryCount)

				cacheEntryCount, err := s.cache.DBSize(s.ctx).Result()
				s.Require().NoError(err)
				s.Assert().Zero(cacheEntryCount)

			} else {
				s.Assert().Equal(tt.expectedError, response["error"])
			}
		})
	}
}

func (s *TestSuite) TestGetProducts() {
	testCases := []struct {
		name           string
		paginated      bool
		limit          string
		offset         string
		setupProducts  bool
		expectedStatus int
		expectedResult []domain.Product
		expectedError  string
	}{
		{
			name:          "get all products - success",
			setupProducts: true,
			expectedResult: []domain.Product{
				domain.Product{
					Id:             1,
					Name:           "Test product #1",
					AdditionalInfo: "Test product #1 info",
				},
				domain.Product{
					Id:             3,
					Name:           "Test product #3",
					AdditionalInfo: "Test product #3 info",
				},
				domain.Product{
					Id:             7,
					Name:           "Test product #7",
					AdditionalInfo: "Test product #7 info",
				},
				domain.Product{
					Id:             11,
					Name:           "Test product #11",
					AdditionalInfo: "Test product #11 info",
				},
				domain.Product{
					Id:             12,
					Name:           "Test product #12",
					AdditionalInfo: "Test product #12 info",
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "get products paged - success",
			paginated:      true,
			limit:          "2",
			offset:         "2",
			setupProducts:  true,
			expectedStatus: http.StatusOK,
			expectedResult: []domain.Product{
				domain.Product{
					Id:             7,
					Name:           "Test product #7",
					AdditionalInfo: "Test product #7 info",
				},
				domain.Product{
					Id:             11,
					Name:           "Test product #11",
					AdditionalInfo: "Test product #11 info",
				},
			},
		},
		{
			name:           "get products paged - invalid limit",
			paginated:      true,
			limit:          "invalid-limit",
			offset:         "20",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid limit",
		},
		{
			name:           "get products paged - invalid offset",
			paginated:      true,
			limit:          "20",
			offset:         "-92",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid offset",
		},
		{
			name:           "get products - db disconnected",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Internal server error",
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			if tt.setupProducts {
				_, err := s.db.Query(`
					INSERT INTO products (id, name, additional_info) VALUES
					(1, 'Test product #1', 'Test product #1 info'),
					(3, 'Test product #3', 'Test product #3 info'),
					(7, 'Test product #7', 'Test product #7 info'),
					(11, 'Test product #11', 'Test product #11 info'),
					(12, 'Test product #12', 'Test product #12 info')
				`)
				s.Require().NoError(err)
			}

			if tt.name == "get products - db disconnected" {
				err := s.pgContainer.Stop(s.ctx, nil)
				require.NoError(s.T(), err)
				s.pgContainerAlive = false
			}

			var resp *http.Response
			var err error
			if tt.paginated {
				resp, err = s.makeRequest("GET", fmt.Sprintf("/products?limit=%s&offset=%s", tt.limit, tt.offset), nil)
				defer resp.Body.Close()
			} else {
				resp, err = s.makeRequest("GET", "/products", nil)
				defer resp.Body.Close()
			}
			s.Assert().Equal(tt.expectedStatus, resp.StatusCode)

			if tt.expectedStatus == http.StatusOK {
				var response []domain.Product
				err = json.NewDecoder(resp.Body).Decode(&response)
				require.NoError(s.T(), err)
				s.Assert().Equal(tt.expectedResult, response)
			} else {
				var response map[string]string
				err = json.NewDecoder(resp.Body).Decode(&response)
				require.NoError(s.T(), err)
				s.Assert().Equal(tt.expectedError, response["error"])
			}

		})
	}
}
