package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/pelyams/simpler_go_service/internal/domain"
	"github.com/pelyams/simpler_go_service/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProductRepoTestSuite struct {
	suite.Suite
	pgContainer *testhelpers.PostgresContainer
	repository  *PostgresRepository
	ctx         context.Context
}

func (suite *ProductRepoTestSuite) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *ProductRepoTestSuite) SetupTest() {
	t := suite.T()
	pgContainer, err := testhelpers.CreatePostgresContainer(suite.ctx)
	if err != nil {
		t.Fatal("failed to create PostgresContainer: ", err)
	}
	suite.pgContainer = pgContainer
	databaseClient, err := sql.Open("postgres", pgContainer.ConnectionString)
	if err != nil {
		t.Fatal("failed to start database client: ", err)
	}

	repository := NewPostgresRepository(databaseClient)
	if err != nil {
		t.Fatal("failed to initailize PostgresRepository intance: ", err)
	}
	suite.repository = repository
}

func (suite *ProductRepoTestSuite) TearDownTest() {
	if err := suite.pgContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatal("error terminating postgres container: ", err)
	}
}

func (suite *ProductRepoTestSuite) SetupSubTest() {
	_, err := suite.repository.db.Exec("TRUNCATE TABLE products")
	if err != nil {
		suite.T().Fatal("failed truncating table: ", err)
	}
}

func TestCustomerRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ProductRepoTestSuite))
}

func (suite *ProductRepoTestSuite) TestGetProduct() {
	testCases := []struct {
		name          string
		testId        int64
		setProduct    bool
		testProduct   domain.Product
		expectedError error
	}{
		{
			name:       "get product from db - success",
			testId:     44,
			setProduct: true,
			testProduct: domain.Product{
				Id:             int64(44),
				Name:           "Product to be retrieved",
				AdditionalInfo: "Additional description",
			},
		},
		{
			name:          "get product from db - not found",
			testId:        3400,
			expectedError: domain.ErrNotFound,
		},
		{
			name:          "get product from db - db disconnected",
			testId:        13,
			expectedError: domain.ErrInternalDb,
		},
	}
	t := suite.T()

	for _, tt := range testCases {
		suite.Run(tt.name, func() {
			if tt.setProduct {
				err := suite.repository.db.QueryRow(
					"INSERT INTO products (id, name, additional_info) VALUES ($1, $2, $3)",
					tt.testProduct.Id,
					tt.testProduct.Name,
					tt.testProduct.AdditionalInfo,
				).Err()
				if err != nil {
					t.Fatal("failed to insert test product", err)
				}
			}

			if tt.name == "get product from db - db disconnected" {
				if err := suite.pgContainer.Stop(suite.ctx, nil); err != nil {
					t.Fatal("failed to stop postgres container")
				}
			}
			product, err := suite.repository.GetProduct(suite.ctx, tt.testId)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.testProduct, *product)
			} else {
				assert.Nil(t, product)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError))
			}
		})
	}
}

func (suite *ProductRepoTestSuite) TestGetAllProducts() {
	t := suite.T()

	results, err := suite.repository.GetAllProducts(suite.ctx)
	assert.NoError(t, err)
	assert.Empty(t, results)

	query := "INSERT INTO products (name, additional_info) VALUES "
	dataLen := 15
	dataToInsert := make([]string, dataLen)
	for i := range dataLen {
		dataToInsert[i] =
			fmt.Sprintf("('Product #%d', 'Description for product #%d')", i, i)
	}
	query = fmt.Sprintf("%s %s", query, strings.Join(dataToInsert, ", "))
	_, err = suite.repository.db.Query(query)
	if err != nil {
		t.Fatal("failed to insert multiple test products into repository: ", err)
	}

	results, err = suite.repository.GetAllProducts(suite.ctx)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, dataLen, len(results))
	assert.Equal(t, "Product #14", results[len(results)-1].Name)

	//here we test "disconnected scenario"
	err = suite.pgContainer.Stop(suite.ctx, nil)
	require.NoError(t, err)

	results, err = suite.repository.GetAllProducts(suite.ctx)
	assert.Nil(t, results)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalDb))
}

func (suite *ProductRepoTestSuite) TestGetAllProductsPaged() {
	t := suite.T()

	results, err := suite.repository.GetProductsPaged(suite.ctx, 8, 0)
	assert.NoError(t, err)
	assert.Empty(t, results)

	query := "INSERT INTO products (name, additional_info) VALUES "
	dataLen := 15
	dataToInsert := make([]string, dataLen)
	for i := range dataLen {
		dataToInsert[i] =
			fmt.Sprintf("('Product #%d', 'Description for product #%d')", i, i)
	}
	query = fmt.Sprintf("%s %s", query, strings.Join(dataToInsert, ", "))
	_, err = suite.repository.db.Query(query)
	if err != nil {
		t.Fatal("failed to insert multiple products into repository: ", err)
	}

	var limit int64 = 8
	var offset int64 = 0
	results, err = suite.repository.GetProductsPaged(suite.ctx, limit, offset)
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, limit, int64(len(results)))
	assert.Equal(t, "Product #1", results[1].Name)

	offset = 8
	results, err = suite.repository.GetProductsPaged(suite.ctx, limit, offset)
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, 7, len(results))
	assert.Equal(t, "Product #8", results[0].Name)

	//disconnected
	err = suite.pgContainer.Stop(suite.ctx, nil)
	require.NoError(t, err)

	results, err = suite.repository.GetProductsPaged(suite.ctx, limit, offset)
	assert.Nil(t, results)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalDb))
}

func (suite *ProductRepoTestSuite) TestStoreProduct() {
	t := suite.T()
	testProduct := domain.NewProduct{
		Name:           "Newly stored product",
		AdditionalInfo: "Newly stored additional desription",
	}
	id, err := suite.repository.StoreProduct(suite.ctx, testProduct)

	require.NoError(t, err)
	var storedId int64
	var storedName string
	var storedInfo string
	err = suite.repository.db.QueryRow("SELECT * FROM products WHERE id=$1", id).Scan(&storedId, &storedName, &storedInfo)
	if err != nil {
		t.Fatal("failed to retrieve products from repository: ", err)
	}
	assert.Equal(t, storedId, id)
	assert.Equal(t, storedName, testProduct.Name)
	assert.Equal(t, storedInfo, testProduct.AdditionalInfo)

	//disconnected
	err = suite.pgContainer.Stop(suite.ctx, nil)
	require.NoError(t, err)

	id, err = suite.repository.StoreProduct(suite.ctx, testProduct)
	assert.Zero(t, id)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInternalDb))

}

func (suite *ProductRepoTestSuite) TestUpdateProductById() {
	testCases := []struct {
		name          string
		testId        int64
		setProduct    bool
		oldProduct    domain.Product
		newProduct    domain.NewProduct
		expectedError error
	}{
		{
			name:       "update product in db - success",
			testId:     7890,
			setProduct: true,
			oldProduct: domain.Product{
				Id:             int64(7890),
				Name:           "Product to be updated",
				AdditionalInfo: "Additional description for old product",
			},
			newProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Info for updated product",
			},
		},
		{
			name:   "update product in db - not found",
			testId: 85,
			newProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Info for updated product",
			},
			expectedError: domain.ErrNotFound,
		},
		{
			name:   "update product in db - db disconnected",
			testId: 13,
			newProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Info for updated product",
			},
			expectedError: domain.ErrInternalDb,
		},
	}
	t := suite.T()
	for _, tt := range testCases {
		suite.Run(tt.name, func() {
			if tt.setProduct {
				err := suite.repository.db.QueryRow("INSERT INTO products (id, name, additional_info) VALUES ($1, $2, $3)", tt.oldProduct.Id, tt.oldProduct.Name, tt.oldProduct.AdditionalInfo).Err()
				if err != nil {
					t.Fatal("failed to insert test product to repository: ", err)
				}
			}

			if tt.name == "update product in db - db disconnected" {
				if err := suite.pgContainer.Stop(suite.ctx, nil); err != nil {
					t.Fatal("failed to stop postgres container")
				}
			}
			olderProduct, err := suite.repository.UpdateProductById(suite.ctx, tt.testId, tt.newProduct)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, olderProduct)
				assert.Equal(t, tt.oldProduct, *olderProduct)
			} else {
				assert.Nil(t, olderProduct)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError))
			}
		})
	}

}

func (suite *ProductRepoTestSuite) TestDeleteProductById() {
	testCases := []struct {
		name          string
		testId        int64
		setProduct    bool
		testProduct   domain.Product
		expectedError error
	}{
		{
			name:       "delete product from db - success",
			testId:     717,
			setProduct: true,
			testProduct: domain.Product{
				Id:             int64(717),
				Name:           "Product to be deleted",
				AdditionalInfo: "Additional description",
			},
		},
		{
			name:          "delete product from db - not found",
			testId:        888,
			expectedError: domain.ErrNotFound,
		},
		{
			name:          "delete product from db - db disconnected",
			testId:        13,
			expectedError: domain.ErrInternalDb,
		},
	}
	t := suite.T()
	for _, tt := range testCases {
		suite.Run(tt.name, func() {
			if tt.setProduct {
				err := suite.repository.db.QueryRow("INSERT INTO products (id, name, additional_info) VALUES ($1, $2, $3)", tt.testProduct.Id, tt.testProduct.Name, tt.testProduct.AdditionalInfo).Err()
				if err != nil {
					t.Fatal("failed to insert test product to repository: ", err)
				}
			}

			if tt.name == "delete product from db - db disconnected" {
				if err := suite.pgContainer.Stop(suite.ctx, nil); err != nil {
					t.Fatal("failed to stop postgres container")
				}
			}
			deletedProduct, err := suite.repository.DeleteProductById(suite.ctx, tt.testId)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, deletedProduct)
				assert.Equal(t, tt.testProduct, *deletedProduct)
			} else {
				assert.Nil(t, deletedProduct)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError))
			}
		})
	}
}

func (suite *ProductRepoTestSuite) TestDeleteAllProducts() {
	testCases := []struct {
		name             string
		setTestProducts  bool
		testProductCount int64
		expectedError    error
	}{
		{
			name:             "delete all products from db - success",
			setTestProducts:  true,
			testProductCount: 151,
		},
		{
			name:             "delete all products from db - db is empty - success",
			testProductCount: 0,
		},
		{
			name:          "delete all products from db - db disconnected",
			expectedError: domain.ErrInternalDb,
		},
	}

	t := suite.T()
	for _, tt := range testCases {
		suite.Run(tt.name, func() {
			if tt.setTestProducts {
				query := "INSERT INTO products (name, additional_info) VALUES "

				dataToInsert := make([]string, tt.testProductCount)
				for i := range tt.testProductCount {
					dataToInsert[i] =
						fmt.Sprintf("('Product #%d', 'Description for product #%d')", i, i)
				}
				query = fmt.Sprintf("%s %s", query, strings.Join(dataToInsert, ", "))
				_, err := suite.repository.db.Query(query)
				if err != nil {
					t.Fatal("failed to insert multiple products into repository", err)
				}
			}

			if tt.name == "delete all products from db - db disconnected" {
				if err := suite.pgContainer.Stop(suite.ctx, nil); err != nil {
					t.Fatal("failed to stop postgres container")
				}
			}

			result, err := suite.repository.DeleteAllProducts(suite.ctx)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.testProductCount, result)

				rows, err := suite.repository.db.Query("SELECT * FROM products")
				if err != nil {
					t.Fatal("failed to retrieve products from repository: ", err)
				}
				defer rows.Close()
				assert.Empty(t, rows.Next())
			} else {
				assert.Zero(t, result)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError))
			}
		})
	}
}
