# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY migrations /app/migrations

EXPOSE 8081
CMD ["/app/api"]
