package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pelyams/simpler_go_service/internal/domain"
	"github.com/pelyams/simpler_go_service/internal/ports"
)

type ProductHandler struct {
	svc ports.ResourseService
}

func NewProductHandler(svc ports.ResourseService) *ProductHandler {
	return &ProductHandler{
		svc: svc,
	}
}

func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	offset := r.URL.Query().Get("offset")
	limit := r.URL.Query().Get("limit")

	// if both offset and limit are provided, pagination is used
	if offset != "" && limit != "" {
		offsetInt, err := parseAndValidate(offset, 1, "offset", r.Context().Value("errorContainer").(*domain.ErrorContainer), w)
		if err != nil {
			return
		}
		limitInt, err := parseAndValidate(limit, 1, "limit", r.Context().Value("errorContainer").(*domain.ErrorContainer), w)
		if err != nil {
			return
		}

		products, serviceErr := h.svc.GetProductsPaged(r.Context(), offsetInt, limitInt)
		if serviceErr != nil {
			storeServiceErrToCtx(r.Context(), serviceErr)
			if serviceErr.CriticalError != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
				return
			}

		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(products)
		return
	}

	// if no pagination parameters, or they are presented partially🥴, return all products
	products, serviceErr := h.svc.GetAllProducts(r.Context())
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req domain.NewProduct
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&req)
	var err error
	switch {
	case decodeErr != nil:
		err = fmt.Errorf("failed to decode payload: %w", decodeErr)
	case req.Name == "" || req.AdditionalInfo == "":
		err = errors.New("failed to decode payload: product name or additional info is empty")
	}
	if err != nil {
		errContainer := r.Context().Value("errorContainer").(*domain.ErrorContainer)
		errContainer.Add(err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	res, serviceErr := h.svc.CreateProduct(r.Context(), req)
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}
	productId := struct {
		ID int64 `json:"id"`
	}{
		ID: res,
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(productId)
}

func (h *ProductHandler) GetProductById(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := strings.TrimPrefix(r.URL.Path, "/product/")
	id, err := parseAndValidate(idStr, 0, "product id", r.Context().Value("errorContainer").(*domain.ErrorContainer), w)
	if err != nil {
		return
	}
	product, serviceErr := h.svc.GetProductById(r.Context(), id)
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			if errors.Is(serviceErr.CriticalError, domain.ErrNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Product not found"})
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write(product)
}

func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := strings.TrimPrefix(r.URL.Path, "/product/")

	id, err := parseAndValidate(idStr, 0, "product id", r.Context().Value("errorContainer").(*domain.ErrorContainer), w)
	if err != nil {
		return
	}
	var req domain.NewProduct
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&req)
	switch {
	case decodeErr != nil:
		err = fmt.Errorf("failed to decode payload: %w", decodeErr)
	case req.Name == "" || req.AdditionalInfo == "":
		err = errors.New("failed to decode payload: product name or additional info is empty")
	}
	if err != nil {
		errContainer := r.Context().Value("errorContainer").(*domain.ErrorContainer)
		errContainer.Add(err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}
	product, serviceErr := h.svc.UpdateProductById(r.Context(), id, req)
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			if errors.Is(serviceErr.CriticalError, domain.ErrNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Product not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := strings.TrimPrefix(r.URL.Path, "/product/")

	id, err := parseAndValidate(idStr, 0, "product id", r.Context().Value("errorContainer").(*domain.ErrorContainer), w)
	if err != nil {
		return
	}
	deletedProduct, serviceErr := h.svc.DeleteProductById(r.Context(), id)
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			if errors.Is(serviceErr.CriticalError, domain.ErrNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Product not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(deletedProduct)

}

func (h *ProductHandler) DeleteAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	deletedRows, serviceErr := h.svc.DeleteAllProducts(r.Context())
	if serviceErr != nil {
		storeServiceErrToCtx(r.Context(), serviceErr)
		if serviceErr.CriticalError != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
			return
		}
	}
	deletedCount := struct {
		DeletedRows int64 `json:"deletedRows"`
	}{
		DeletedRows: deletedRows,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(deletedCount)

}

func parseAndValidate(s string, lb int64, name string, c *domain.ErrorContainer, w http.ResponseWriter) (int64, error) {
	value, parseErr := strconv.ParseInt(s, 10, 64)
	var err error
	switch {
	case parseErr != nil:
		err = fmt.Errorf("handler error: failed to parse %s: %w", name, parseErr)
	case value < lb:
		err = errors.New(fmt.Sprintf("hanlder error: invalid %s: has value %d, must be ge %d", name, value, lb))
	}
	if err != nil {
		c.Add(err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Invalid %s", name)})
		return 0, errors.New(fmt.Sprintf("failed to get valid value while parsing"))
	}
	return value, nil
}

func storeServiceErrToCtx(ctx context.Context, e *domain.ServiceError) {
	errs := ctx.Value("errorContainer").(*domain.ErrorContainer)
	if e.CriticalError != nil {
		errs.Add(e.CriticalError)
	}
	if e.NonCriticalErrors != nil {
		errs.Add(e.NonCriticalErrors...)
	}
}
