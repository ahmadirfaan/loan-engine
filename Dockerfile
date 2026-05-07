FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /loan-engine ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /loan-engine /usr/local/bin/loan-engine

EXPOSE 8080

CMD ["loan-engine"]
