package routing

import (
	"net/http"
)

type Router struct {
	handler *ProductHandler
}

func NewRouter(handler *ProductHandler) *Router {
	return &Router{
		handler: handler,
	}
}

func (router *Router) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			router.handler.GetProducts(w, r)
		case http.MethodDelete:
			router.handler.DeleteAll(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			router.handler.CreateProduct(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	})

	mux.HandleFunc("/product/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			router.handler.GetProductById(w, r)
		case http.MethodPut:
			router.handler.UpdateProduct(w, r)
		case http.MethodDelete:
			router.handler.DeleteProduct(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}
