package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pelyams/simpler_go_service/internal/domain"
	"github.com/pelyams/simpler_go_service/internal/ports"
)

type ResourseService struct {
	db    ports.Repository
	cache ports.Cache
}

func NewResourceService(db ports.Repository, cache ports.Cache) *ResourseService {
	return &ResourseService{
		db:    db,
		cache: cache,
	}
}

func (s *ResourseService) GetProductById(ctx context.Context, id int64) ([]byte, *domain.ServiceError) {
	var nonCriticalErrors []error
	cacheRes, cacheErr := s.cache.GetJSONProductById(ctx, id)
	if cacheErr == nil {
		return cacheRes, nil
	} else {
		nonCriticalErrors = append(nonCriticalErrors, cacheErr)
	}
	dbRes, dbErr := s.db.GetProduct(ctx, id)
	if dbErr != nil {
		return nil, domain.NewServiceError(dbErr, nonCriticalErrors)
	}

	err := s.cache.SetProduct(ctx, dbRes)
	if err != nil {
		nonCriticalErrors = append(nonCriticalErrors, err)
	}
	res, err := json.Marshal(dbRes)
	if err != nil {
		marshallingErr := fmt.Errorf("service layer error: %w", err)
		nonCriticalErrors = append(nonCriticalErrors, marshallingErr)
		return nil, domain.NewServiceError(marshallingErr, nonCriticalErrors)
	}
	if nonCriticalErrors != nil {
		return res, domain.NewServiceError(nil, nonCriticalErrors)
	}
	return res, nil
}

func (s *ResourseService) GetAllProducts(ctx context.Context) ([]domain.Product, *domain.ServiceError) {
	products, err := s.db.GetAllProducts(ctx)
	if err != nil {
		return nil, domain.NewServiceError(err, nil)
	}
	return products, nil
}

func (s *ResourseService) GetProductsPaged(ctx context.Context, limit int64, offset int64) ([]domain.Product, *domain.ServiceError) {

	products, err := s.db.GetProductsPaged(ctx, limit, offset)
	if err != nil {
		return nil, domain.NewServiceError(err, nil)
	}
	return products, nil
}

func (s *ResourseService) CreateProduct(ctx context.Context, product domain.NewProduct) (int64, *domain.ServiceError) {
	id, dbErr := s.db.StoreProduct(ctx, product)
	if dbErr != nil {
		return 0, domain.NewServiceError(dbErr, nil)
	}

	//lets set product to cache as well for no reason
	//assuming cache access is fast
	newlyStoredProduct := domain.Product{
		Id: id, Name: product.Name, AdditionalInfo: product.AdditionalInfo,
	}
	cacheErr := s.cache.SetProduct(ctx, &newlyStoredProduct)
	if cacheErr != nil {
		return id, domain.NewServiceError(nil, []error{cacheErr})
	}
	return id, nil
}

func (s *ResourseService) UpdateProductById(ctx context.Context, id int64, product domain.NewProduct) (*domain.Product, *domain.ServiceError) {
	var nonCriticalErrors []error
	cacheErr := s.cache.DeleteProductById(ctx, id)
	if cacheErr != nil {
		if errors.Is(cacheErr, domain.ErrNotFound) {
			nonCriticalErrors = append(nonCriticalErrors, cacheErr)
		} else {
			return nil, domain.NewServiceError(cacheErr, nil)
		}
	}
	oldProduct, dbErr := s.db.UpdateProductById(ctx, id, product)
	if dbErr != nil {
		return nil, domain.NewServiceError(dbErr, nonCriticalErrors)
	}
	if nonCriticalErrors != nil {
		return oldProduct, domain.NewServiceError(nil, nonCriticalErrors)
	}
	return oldProduct, nil
}

func (s *ResourseService) DeleteProductById(ctx context.Context, id int64) (*domain.Product, *domain.ServiceError) {
	var nonCriticalErrors []error
	cacheErr := s.cache.DeleteProductById(ctx, id)
	if cacheErr != nil {
		if errors.Is(cacheErr, domain.ErrNotFound) {
			nonCriticalErrors = append(nonCriticalErrors, cacheErr)
		} else {
			return nil, domain.NewServiceError(cacheErr, nil)
		}
	}
	deletedProduct, dbErr := s.db.DeleteProductById(ctx, id)
	if dbErr != nil {
		return nil, domain.NewServiceError(dbErr, nonCriticalErrors)
	}
	if nonCriticalErrors != nil {
		return deletedProduct, domain.NewServiceError(nil, nonCriticalErrors)
	}
	return deletedProduct, nil
}

func (s *ResourseService) DeleteAllProducts(ctx context.Context) (int64, *domain.ServiceError) {
	cacheErr := s.cache.ClearCache(ctx)
	if cacheErr != nil {
		return 0, domain.NewServiceError(cacheErr, nil)
	}

	rowsDeleted, dbErr := s.db.DeleteAllProducts(ctx)
	if dbErr != nil {
		return 0, domain.NewServiceError(dbErr, nil)
	}
	return rowsDeleted, nil
}
