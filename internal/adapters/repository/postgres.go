package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
	var product domain.Product
	err := r.db.QueryRow("SELECT id, name, additional_info FROM products WHERE id = $1", id).
		Scan(&product.Id, &product.Name, &product.AdditionalInfo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: failed to find product %d in DB", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: failed to get product %d. %s", domain.ErrInternalDb, id, err.Error())
	}
	return &product, nil
}

func (r *PostgresRepository) GetAllProducts(ctx context.Context) ([]domain.Product, error) {
	var products = make([]domain.Product, 0)
	rows, err := r.db.Query("SELECT id, name, additional_info FROM products")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get all products", domain.ErrInternalDb)
	}
	defer rows.Close()
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(&product.Id, &product.Name, &product.AdditionalInfo); err != nil {
			return nil, fmt.Errorf("%w: failed to convert row into go type. %s", domain.ErrInternalDb, err.Error())
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: error while iterating over rows. %s", domain.ErrInternalDb, err.Error())
	}
	return products, nil
}

func (r *PostgresRepository) GetProductsPaged(ctx context.Context, limit int64, offset int64) ([]domain.Product, error) {
	var products = make([]domain.Product, 0, limit)
	rows, err := r.db.Query("SELECT id, name, additional_info FROM products LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get paginated products. %s", domain.ErrInternalDb, err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(&product.Id, &product.Name, &product.AdditionalInfo); err != nil {
			return nil, fmt.Errorf("%w: failed to convert row into go type. %s", domain.ErrInternalDb, err.Error())
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: error while iterating over rows. %s", domain.ErrInternalDb, err.Error())
	}
	return products, nil
}

func (r *PostgresRepository) UpdateProductById(ctx context.Context, id int64, product domain.NewProduct) (*domain.Product, error) {
	var oldProduct domain.Product
	err := r.db.QueryRow(
		`UPDATE products SET name = $1, additional_info = $2
		FROM (SELECT name, additional_info FROM products WHERE id = $3) as old
		WHERE id = $3
		RETURNING id, old.name, old.additional_info`,
		product.Name, product.AdditionalInfo, id).Scan(&oldProduct.Id, &oldProduct.Name, &oldProduct.AdditionalInfo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: failed to find product %d in DB", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: failed to update product %d. %s", domain.ErrInternalDb, id, err.Error())
	}
	return &oldProduct, nil
}

func (r *PostgresRepository) DeleteProductById(ctx context.Context, id int64) (*domain.Product, error) {
	var oldProduct domain.Product
	err := r.db.QueryRow("DELETE FROM products WHERE id = $1 RETURNING id, name, additional_info", id).Scan(&oldProduct.Id, &oldProduct.Name, &oldProduct.AdditionalInfo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: failed to find product %d in DB", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: failed to delete product %d. %s", domain.ErrInternalDb, id, err.Error())
	}
	return &oldProduct, nil
}

func (r *PostgresRepository) DeleteAllProducts(ctx context.Context) (int64, error) {
	var count int64
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("%w: failed to start transaction. %s", domain.ErrInternalDb, err.Error())
	}
	defer tx.Rollback()

	err = tx.QueryRow("SELECT COUNT (*) FROM products").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to count rows. %s", domain.ErrInternalDb, err.Error())
	}
	_, err = tx.Exec("TRUNCATE TABLE products")
	if err != nil {
		return 0, fmt.Errorf("%w: failed to truncate table. %s", domain.ErrInternalDb, err.Error())
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("%w: failed to commit transaction. %s", domain.ErrInternalDb, err.Error())
	}

	return count, nil
}

func (r *PostgresRepository) StoreProduct(ctx context.Context, product domain.NewProduct) (int64, error) {
	var id int64
	err := r.db.QueryRow("INSERT INTO products (name, additional_info) VALUES ($1, $2) RETURNING id", product.Name, product.AdditionalInfo).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to store product. %s", domain.ErrInternalDb, err.Error())
	}
	return id, nil
}
