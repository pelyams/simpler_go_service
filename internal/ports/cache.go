package ports

import (
	"context"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type Cache interface {
	SetProduct(ctx context.Context, product *domain.Product) error
	GetJSONProductById(ctx context.Context, id int64) ([]byte, error)
	DeleteProductById(ctx context.Context, id int64) error
	ClearCache(ctx context.Context) error
}
