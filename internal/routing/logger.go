package routing

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pelyams/simpler_go_service/internal/domain"
)

type Logger struct {
	requestCount uint64
	file         *os.File
	logger       *log.Logger
}

func NewLogger(startingRequestId uint64, fileName string) (*Logger, error) {
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	mw := io.MultiWriter(file, os.Stdout)
	logger := log.New(mw, "", log.LstdFlags)
	return &Logger{
		requestCount: startingRequestId,
		file:         file,
		logger:       logger,
	}, nil
}

func (l *Logger) Close() {
	l.file.Close()
}

func (l *Logger) getNewRequestId() uint64 {
	request_id := l.requestCount
	l.requestCount++
	return request_id
}

func (l *Logger) LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req_id := l.getNewRequestId()
		method := r.Method
		path := r.URL.Path
		started := time.Now()
		errContainer := domain.NewErrorContainer()
		ctx := context.WithValue(r.Context(), "errorContainer", &errContainer)
		next.ServeHTTP(w, r.WithContext(ctx))
		duration := time.Since(started)
		body := "none"
		if method != "GET" && method != "DELETE" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				loggerErr := fmt.Errorf("logger error: failed to read request body: %v", err)
				errs := ctx.Value("errorContainer").(*domain.ErrorContainer)
				errs.Add(loggerErr)
				body = "failed to read body"
			} else {
				defer r.Body.Close()
				body = string(bodyBytes)
			}
		}
		if errs := ctx.Value("errorContainer").(*domain.ErrorContainer); errs != nil && len(errs.Unwrap()) > 0 {
			l.logger.Printf(
				"Request: %d | ERROR | Method: %s | Path: %s | Body: %s | Duration: %v | Error(s):\n",
				req_id,
				method,
				path,
				body,
				duration)
			for i, error := range errs.Unwrap() {
				l.logger.Printf(" %d. %v\n", i+1, error)
			}
			return
		} else {
			l.logger.Printf(
				"Request: %d | OK | Method: %s | Path: %s | Body: %s | Duration: %v\n",
				req_id,
				method,
				path,
				body,
				duration)
		}
	})
}
