FROM golang:alpine AS builder

RUN apk add --no-cache git ca-certificates
WORKDIR /src

COPY protos /protos

COPY cmd/match-adapter/go.mod cmd/match-adapter/go.sum ./
RUN go mod download -x
COPY cmd/match-adapter/ ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/match-adapter ./cmd

FROM alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=builder /app/match-adapter /app/match-adapter
COPY --from=builder /src/config ./config
COPY --from=builder /src/.env .env

EXPOSE 44045
ENTRYPOINT ["/app/match-adapter"]
CMD ["--config=/app/config/local.yaml"]