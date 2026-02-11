FROM golang:alpine AS builder

RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY protos /protos

COPY cmd/airfare-provider/go.mod cmd/airfare-provider/go.sum ./
RUN go mod download -x
COPY cmd/airfare-provider/ ./

RUN CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o /app/airfare-provider ./cmd

FROM alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=builder /app/airfare-provider /app/airfare-provider
COPY --from=builder /src/config ./config
COPY --from=builder /src/.env .env

EXPOSE 44044
ENTRYPOINT ["/app/airfare-provider"]
CMD ["--config=/app/config/local.yaml"]
