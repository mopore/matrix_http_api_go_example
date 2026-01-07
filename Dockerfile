FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o matrixbot ./cmd/matrixbot

FROM alpine:latest
RUN apk --no-cache add ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /app/matrixbot /app/matrixbot
USER appuser
ENTRYPOINT ["/app/matrixbot"]
