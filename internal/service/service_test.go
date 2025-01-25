package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Product), args.Error(1)
}

func (m *MockRepository) GetAllProducts(ctx context.Context) ([]domain.Product, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Product), args.Error(1)
}

func (m *MockRepository) GetProductsPaged(ctx context.Context, limit int64, offset int64) ([]domain.Product, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]domain.Product), args.Error(1)
}

func (m *MockRepository) StoreProduct(ctx context.Context, product domain.NewProduct) (int64, error) {
	args := m.Called(ctx, product)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) UpdateProductById(ctx context.Context, id int64, product domain.NewProduct) (*domain.Product, error) {
	args := m.Called(ctx, id, product)
	return args.Get(0).(*domain.Product), args.Error(1)
}

func (m *MockRepository) DeleteProductById(ctx context.Context, id int64) (*domain.Product, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*domain.Product), args.Error(1)
}

func (m *MockRepository) DeleteAllProducts(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) SetProduct(ctx context.Context, product *domain.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}
func (m *MockCache) GetJSONProductById(ctx context.Context, id int64) ([]byte, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockCache) DeleteProductById(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockCache) ClearCache(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type ServiceTestSuite struct {
	suite.Suite
	service        *ResourseService
	ctx            context.Context
	mockRepository *MockRepository
	mockCache      *MockCache
}

func (suite *ServiceTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	mockPostgres := new(MockRepository)
	mockRedis := new(MockCache)
	suite.service = &ResourseService{db: mockPostgres, cache: mockRedis}
	suite.mockRepository = mockPostgres
	suite.mockCache = mockRedis
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupSubTest() {
	suite.mockRepository.ExpectedCalls = nil
	suite.mockCache.ExpectedCalls = nil
}

func (suite *ServiceTestSuite) TearDownSubTest() {
	suite.mockCache.AssertExpectations(suite.T())
	suite.mockRepository.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetProductById() {

	testCases := []struct {
		name      string
		productId int64

		expectedResult []byte
		expectedError  error
		setupMocks     func()
	}{
		{
			name:           "Product found in cache",
			productId:      1,
			expectedResult: []byte(`{"id":1, "name":"Cached Product", "additionalInfo":"Additional info for cached product"}`),
			expectedError:  nil,
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(1)).Return([]byte(`{"id":1, "name":"Cached Product", "additionalInfo":"Additional info for cached product"}`), nil).Once()
				suite.mockRepository.ExpectedCalls = nil
			},
		},
		{
			name:           "Product not in cache but found in storage",
			productId:      2,
			expectedResult: []byte(`{"id":2,"name":"Stored Product","additionalInfo":"Additional info for stored product"}`),
			expectedError:  &domain.ServiceError{CriticalError: nil, NonCriticalErrors: []error{domain.ErrNotFound}},
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(2)).Return([]byte(nil), domain.ErrNotFound).Once()
				suite.mockCache.On("SetProduct", suite.ctx, &domain.Product{
					Id:             int64(2),
					Name:           "Stored Product",
					AdditionalInfo: "Additional info for stored product",
				}).Return(nil).Once()
				suite.mockRepository.On("GetProduct", suite.ctx, int64(2)).Return(&domain.Product{
					Id:             int64(2),
					Name:           "Stored Product",
					AdditionalInfo: "Additional info for stored product",
				}, nil).Once()
			},
		},
		{
			name:           "Product not found in cache or storage",
			productId:      3,
			expectedResult: nil,
			expectedError:  &domain.ServiceError{CriticalError: nil, NonCriticalErrors: []error{domain.ErrNotFound, domain.ErrNotFound}},
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(3)).Return([]byte(nil), domain.ErrNotFound).Once()
				suite.mockRepository.On("GetProduct", suite.ctx, int64(3)).Return((*domain.Product)(nil), domain.ErrNotFound).Once()
			},
		},
		{
			name:           "Product not found in storage, cache returns internal error",
			productId:      4,
			expectedResult: nil,
			expectedError:  &domain.ServiceError{CriticalError: nil, NonCriticalErrors: []error{domain.ErrInternalCache, domain.ErrNotFound}},
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(4)).Return([]byte(nil), domain.ErrInternalCache).Once()
				suite.mockRepository.On("GetProduct", suite.ctx, int64(4)).Return((*domain.Product)(nil), domain.ErrNotFound).Once()
			},
		},
		{
			name:           "Product found in storage, cache returns internal error",
			productId:      5,
			expectedResult: []byte(`{"id":5,"name":"Stored Product","additionalInfo":"Additional info for stored product"}`),
			expectedError:  &domain.ServiceError{CriticalError: nil, NonCriticalErrors: []error{domain.ErrInternalCache}},
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(5)).Return([]byte(nil), domain.ErrInternalCache).Once()
				suite.mockCache.On("SetProduct", suite.ctx, &domain.Product{
					Id:             int64(5),
					Name:           "Stored Product",
					AdditionalInfo: "Additional info for stored product",
				}).Return(nil).Once()
				suite.mockRepository.On("GetProduct", suite.ctx, int64(5)).Return(&domain.Product{
					Id: 5, Name: "Stored Product",
					AdditionalInfo: "Additional info for stored product"}, nil).Once()
			},
		},
		{
			name:           "Repo and cache return internal errors",
			productId:      6,
			expectedResult: nil,
			expectedError:  &domain.ServiceError{CriticalError: domain.ErrInternalDb, NonCriticalErrors: []error{domain.ErrInternalCache}},
			setupMocks: func() {
				suite.mockCache.On("GetJSONProductById", suite.ctx, int64(6)).Return([]byte(nil), domain.ErrInternalCache).Once()
				suite.mockRepository.On("GetProduct", suite.ctx, int64(6)).Return((*domain.Product)(nil), domain.ErrInternalDb).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.GetProductById(suite.ctx, tc.productId)
			//annoyingly repeats across all the tests
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}
}

func (suite *ServiceTestSuite) TestGetAllProducts() {
	testCases := []struct {
		name string

		expectedResult []domain.Product
		expectedError  error
		setupMocks     func()
	}{
		{
			name: "Got all products from repository - no errors",
			expectedResult: []domain.Product{
				{Id: 1, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				{Id: 2, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				{Id: 3, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
			},
			expectedError: nil,
			setupMocks: func() {
				suite.mockRepository.On("GetAllProducts", suite.ctx).Return([]domain.Product{
					{Id: 1, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
					{Id: 2, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
					{Id: 3, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				}, nil).Once()
			},
		},
		{
			name:           "Get all products - db error",
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalDb,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockRepository.On("GetAllProducts", suite.ctx).Return([]domain.Product(nil), domain.ErrInternalDb).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.GetAllProducts(suite.ctx)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}
}

func (suite *ServiceTestSuite) TestGetProductsPaged() {
	testCases := []struct {
		name           string
		limit          int64
		offset         int64
		expectedResult []domain.Product
		expectedError  error
		setupMocks     func()
	}{
		{
			name:   "Got products paged - no error",
			limit:  3,
			offset: 3,
			expectedResult: []domain.Product{
				{Id: 4, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				{Id: 5, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				{Id: 6, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
			},
			expectedError: nil,
			setupMocks: func() {
				suite.mockRepository.On("GetProductsPaged", suite.ctx, int64(3), int64(3)).Return([]domain.Product{
					{Id: 4, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
					{Id: 5, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
					{Id: 6, Name: "Stored Product", AdditionalInfo: "Additional info for stored product"},
				}, nil).Once()
			},
		},
		{
			name:           "Get products paged - db error",
			limit:          3,
			offset:         6,
			expectedResult: nil,
			expectedError:  &domain.ServiceError{CriticalError: domain.ErrInternalDb, NonCriticalErrors: nil},
			setupMocks: func() {
				suite.mockRepository.On("GetProductsPaged", suite.ctx, int64(3), int64(6)).Return([]domain.Product(nil), domain.ErrInternalDb).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.GetProductsPaged(suite.ctx, tc.limit, tc.offset)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}

}

func (suite *ServiceTestSuite) TestCreateProduct() {
	testCases := []struct {
		name           string
		product        domain.NewProduct
		cacheError     error
		storageResult  int64
		storageError   error
		expectedResult int64
		expectedError  error
		setupMocks     func()
	}{
		{
			name: "Create product - product added to storage and to cache",
			product: domain.NewProduct{
				Name:           "New product to be stored",
				AdditionalInfo: "Product description",
			},
			cacheError:     nil,
			storageResult:  1,
			storageError:   nil,
			expectedResult: 1,
			expectedError:  nil,
			setupMocks: func() {
				suite.mockRepository.On("StoreProduct",
					suite.ctx,
					domain.NewProduct{
						Name:           "New product to be stored",
						AdditionalInfo: "Product description",
					},
				).Return(int64(1), nil).Once()

				suite.mockCache.On("SetProduct",
					suite.ctx,
					&domain.Product{
						Id:             int64(1),
						Name:           "New product to be stored",
						AdditionalInfo: "Product description",
					},
				).Return(nil).Once()
			},
		},
		{
			name: "Create product - product added to storage, cache internal error",
			product: domain.NewProduct{
				Name:           "New product to be stored",
				AdditionalInfo: "Product description",
			},
			cacheError:     domain.ErrInternalCache,
			storageResult:  2,
			storageError:   nil,
			expectedResult: 2,
			expectedError: &domain.ServiceError{
				CriticalError:     nil,
				NonCriticalErrors: []error{domain.ErrInternalCache},
			},
			setupMocks: func() {
				suite.mockRepository.On("StoreProduct", suite.ctx, domain.NewProduct{Name: "New product to be stored", AdditionalInfo: "Product description"}).Return(int64(2), nil).Once()
				suite.mockCache.On("SetProduct", suite.ctx, &domain.Product{Id: int64(2), Name: "New product to be stored", AdditionalInfo: "Product description"}).Return(domain.ErrInternalCache).Once()
			},
		},
		{
			name: "Create product - db internal error",
			product: domain.NewProduct{
				Name:           "New product to be stored",
				AdditionalInfo: "Product description",
			},
			cacheError:     nil,
			storageResult:  0,
			storageError:   domain.ErrInternalDb,
			expectedResult: 0,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalDb,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockRepository.On("StoreProduct", suite.ctx, domain.NewProduct{Name: "New product to be stored", AdditionalInfo: "Product description"}).Return(int64(0), domain.ErrInternalDb)
			},
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.CreateProduct(suite.ctx, tc.product)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}

}

func (suite *ServiceTestSuite) TestUpdateProductById() {
	testCases := []struct {
		name           string
		productId      int64
		updatedProduct domain.NewProduct
		expectedResult *domain.Product
		expectedError  error
		setupMocks     func()
	}{
		{
			name:      "Product updated - no errors",
			productId: 1,
			updatedProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Updated product description",
			},
			expectedResult: &domain.Product{
				Id:             1,
				Name:           "Old product",
				AdditionalInfo: "Older product description",
			},
			expectedError: nil,
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(1)).Return(nil).Once()
				suite.mockRepository.On("UpdateProductById", suite.ctx, int64(1), domain.NewProduct{
					Name:           "Updated product",
					AdditionalInfo: "Updated product description",
				}).Return(&domain.Product{
					Id:             1,
					Name:           "Old product",
					AdditionalInfo: "Older product description",
				}, nil).Once()
			},
		},
		{
			name:      "Product update - cache returns internal error",
			productId: 2,
			updatedProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Updated product description",
			},
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalCache,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(2)).Return(domain.ErrInternalCache).Once()
			},
		},
		{
			name:      "Product update - db returns internal error",
			productId: 3,
			updatedProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Updated product description",
			},
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalDb,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(3)).Return(nil).Once()
				suite.mockRepository.On("UpdateProductById", suite.ctx, int64(3), domain.NewProduct{
					Name:           "Updated product",
					AdditionalInfo: "Updated product description",
				}).Return((*domain.Product)(nil), domain.ErrInternalDb).Once()
			},
		},
		{
			name:      "Product update - product not found in db",
			productId: 3,
			updatedProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Updated product description",
			},
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrNotFound,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(3)).Return(nil).Once()
				suite.mockRepository.On("UpdateProductById", suite.ctx, int64(3), domain.NewProduct{
					Name:           "Updated product",
					AdditionalInfo: "Updated product description",
				}).Return((*domain.Product)(nil), domain.ErrNotFound).Once()
			},
		},
		{
			name:      "Product update - product not found in cache",
			productId: 4,
			updatedProduct: domain.NewProduct{
				Name:           "Updated product",
				AdditionalInfo: "Updated product description",
			},
			expectedResult: &domain.Product{
				Id:             4,
				Name:           "Old product",
				AdditionalInfo: "Older product description",
			},
			expectedError: &domain.ServiceError{
				CriticalError:     nil,
				NonCriticalErrors: []error{domain.ErrNotFound},
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(4)).Return(domain.ErrNotFound).Once()
				suite.mockRepository.On("UpdateProductById", suite.ctx, int64(4), domain.NewProduct{
					Name:           "Updated product",
					AdditionalInfo: "Updated product description",
				}).Return(&domain.Product{
					Id:             4,
					Name:           "Old product",
					AdditionalInfo: "Older product description",
				}, nil).Once()
			},
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.UpdateProductById(suite.ctx, tc.productId, tc.updatedProduct)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}
}

func (suite *ServiceTestSuite) TestDeleteProductById() {
	testCases := []struct {
		name           string
		productId      int64
		expectedResult *domain.Product
		expectedError  error
		setupMocks     func()
	}{
		{
			name:      "Product deleted - no errors",
			productId: 1,
			expectedResult: &domain.Product{
				Id:             1,
				Name:           "Old product",
				AdditionalInfo: "Older product description",
			},
			expectedError: nil,
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(1)).Return(nil).Once()
				suite.mockRepository.On("DeleteProductById", suite.ctx, int64(1)).Return(&domain.Product{
					Id:             1,
					Name:           "Old product",
					AdditionalInfo: "Older product description",
				}, nil).Once()
			},
		},
		{
			name:           "Product delete - cache returns internal error",
			productId:      2,
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalCache,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(2)).Return(domain.ErrInternalCache).Once()
			},
		},
		{
			name:           "Product delete - db returns internal error",
			productId:      3,
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrInternalDb,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(3)).Return(nil).Once()
				suite.mockRepository.On("DeleteProductById", suite.ctx, int64(3)).Return((*domain.Product)(nil), domain.ErrInternalDb).Once()
			},
		},
		{
			name:           "Product delete - product not found in db",
			productId:      3,
			expectedResult: nil,
			expectedError: &domain.ServiceError{
				CriticalError:     domain.ErrNotFound,
				NonCriticalErrors: nil,
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(3)).Return(nil).Once()
				suite.mockRepository.On("DeleteProductById", suite.ctx, int64(3)).Return((*domain.Product)(nil), domain.ErrNotFound).Once()
			},
		},
		{
			name:      "Product delete - product not found in cache",
			productId: 4,
			expectedResult: &domain.Product{
				Id: 4, Name: "Old product",
				AdditionalInfo: "Older product description",
			},
			expectedError: &domain.ServiceError{
				CriticalError:     nil,
				NonCriticalErrors: []error{domain.ErrNotFound},
			},
			setupMocks: func() {
				suite.mockCache.On("DeleteProductById", suite.ctx, int64(4)).Return(domain.ErrNotFound).Once()
				suite.mockRepository.On("DeleteProductById", suite.ctx, int64(4)).Return(&domain.Product{
					Id: 4, Name: "Old product",
					AdditionalInfo: "Older product description",
				}, nil).Once()
			},
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.DeleteProductById(suite.ctx, tc.productId)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}
}

func (suite *ServiceTestSuite) TestDeleteAllProducts() {
	testCases := []struct {
		name           string
		expectedResult int64
		expectedError  error
		setupMocks     func()
	}{
		{
			name:           "Delete all products - no error",
			expectedResult: int64(155),
			expectedError:  nil,
			setupMocks: func() {
				suite.mockCache.On("ClearCache", suite.ctx).Return(nil).Once()
				suite.mockRepository.On("DeleteAllProducts", suite.ctx).Return(int64(155), nil).Once()
			},
		},
		{
			name:          "Delete all products - cache internal error",
			expectedError: &domain.ServiceError{CriticalError: domain.ErrInternalCache, NonCriticalErrors: nil},
			setupMocks: func() {
				suite.mockCache.On("ClearCache", suite.ctx).Return(domain.ErrInternalCache).Once()
			},
		},
		{
			name:           "Delete all products - cache internal error",
			expectedResult: int64(0),
			expectedError:  &domain.ServiceError{CriticalError: domain.ErrInternalDb, NonCriticalErrors: nil},
			setupMocks: func() {
				suite.mockCache.On("ClearCache", suite.ctx).Return(nil).Once()
				suite.mockRepository.On("DeleteAllProducts", suite.ctx).Return(int64(0), domain.ErrInternalDb).Once()
			},
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()
			result, err := suite.service.DeleteAllProducts(suite.ctx)
			if tc.expectedError != nil {
				suite.Error(err)
				suite.EqualError(err, tc.expectedError.Error())
			} else {
				suite.Nil(err)
			}
			suite.Equal(tc.expectedResult, result)
		})
	}
}
