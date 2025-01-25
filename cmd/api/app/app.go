package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"

	"github.com/redis/go-redis/v9"

	"github.com/pelyams/simpler_go_service/internal/adapters/cache"
	"github.com/pelyams/simpler_go_service/internal/adapters/repository"
	"github.com/pelyams/simpler_go_service/internal/config"
	"github.com/pelyams/simpler_go_service/internal/ports"
	"github.com/pelyams/simpler_go_service/internal/routing"
	"github.com/pelyams/simpler_go_service/internal/service"
)

type App struct {
	config     *config.Config
	db         ports.Repository
	cache      ports.Cache
	service    ports.ResourseService
	handler    *routing.ProductHandler
	router     *http.Handler
	middleware *routing.Logger
}

func New() (*App, error) {
	cfg := config.Load()

	dbConnetionStr := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
		cfg.DatabaseName,
	)
	databaseClient, err := sql.Open("postgres", dbConnetionStr)
	if err != nil {
		log.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost + ":" + cfg.RedisPort,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	redisClient.ConfigSet(context.Background(), "maxmemory", "10mb")
	redisClient.ConfigSet(context.Background(), "maxmemory-policy", "allkeys-lru")

	repo := repository.NewPostgresRepository(databaseClient)
	cache := cache.NewRedisCache(redisClient)
	service := service.NewResourceService(repo, cache)

	handler := routing.NewProductHandler(service)
	router := routing.NewRouter(handler).SetupRoutes()
	logFile := cfg.LogFile
	if logFile == "" {
		logFile = "app.log"
	}

	logger, err := routing.NewLogger(0, logFile)
	if err != nil {
		log.Fatal(err)
	}
	return &App{
		config:     cfg,
		db:         repo,
		cache:      cache,
		service:    service,
		handler:    handler,
		router:     &router,
		middleware: logger,
	}, nil
}

func (a *App) Run() error {
	return http.ListenAndServe(":"+a.config.Port, a.middleware.LoggerMiddleware(*a.router))
}
