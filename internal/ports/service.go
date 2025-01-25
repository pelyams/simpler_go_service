package ports

import (
	"context"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type ResourseService interface {
	GetProductById(ctx context.Context, id int64) ([]byte, *domain.ServiceError)
	GetAllProducts(ctx context.Context) ([]domain.Product, *domain.ServiceError)
	GetProductsPaged(ctx context.Context, limit int64, offset int64) ([]domain.Product, *domain.ServiceError)
	CreateProduct(ctx context.Context, product domain.NewProduct) (int64, *domain.ServiceError)
	UpdateProductById(ctx context.Context, id int64, product domain.NewProduct) (*domain.Product, *domain.ServiceError)
	DeleteProductById(ctx context.Context, id int64) (*domain.Product, *domain.ServiceError)
	DeleteAllProducts(ctx context.Context) (int64, *domain.ServiceError)
}
