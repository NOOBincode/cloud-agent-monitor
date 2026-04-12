# Build from repository root: docker build -f demo-apps/checkout-sim/Dockerfile .
FROM golang:1.24-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/checkout-sim ./demo-apps/checkout-sim

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/checkout-sim /usr/local/bin/checkout-sim
EXPOSE 18080
USER nobody
ENTRYPOINT ["/usr/local/bin/checkout-sim"]
