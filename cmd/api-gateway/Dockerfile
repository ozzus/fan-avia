FROM golang:alpine AS builder

RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY protos /protos

COPY cmd/api-gateway/go.mod cmd/api-gateway/go.sum ./
RUN go mod download -x
COPY cmd/api-gateway/ ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/api-gateway ./cmd

FROM alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=builder /app/api-gateway /app/api-gateway
COPY --from=builder /src/config ./config
COPY --from=builder /src/.env .env

EXPOSE 8080
ENTRYPOINT ["/app/api-gateway"]
CMD ["--config=/app/config/local.yaml"]
