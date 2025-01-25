package ports

import (
	"context"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type Repository interface {
	GetProduct(ctx context.Context, id int64) (*domain.Product, error)
	GetAllProducts(ctx context.Context) ([]domain.Product, error)
	GetProductsPaged(ctx context.Context, limit int64, offset int64) ([]domain.Product, error)
	StoreProduct(ctx context.Context, product domain.NewProduct) (int64, error)
	UpdateProductById(ctx context.Context, id int64, product domain.NewProduct) (*domain.Product, error)
	DeleteProductById(ctx context.Context, id int64) (*domain.Product, error)
	DeleteAllProducts(ctx context.Context) (int64, error)
}
