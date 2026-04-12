FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /platform-api ./cmd/platform-api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /platform-api /app/platform-api
COPY --from=builder /app/configs /app/configs

ENV TZ=Asia/Shanghai

EXPOSE 8080

ENTRYPOINT ["/app/platform-api"]
