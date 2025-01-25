FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/api ./cmd/api
COPY internal ./internal

WORKDIR /app/cmd/api
RUN CGO_ENABLED=0 go build -o /app/main .


FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]
