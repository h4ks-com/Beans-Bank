FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init -g cmd/server/main.go

RUN CGO_ENABLED=1 GOOS=linux go build -a -o beapin ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /root/

COPY --from=builder /app/beapin .
COPY --from=builder /app/web ./web
COPY --from=builder /app/docs ./docs

EXPOSE 8080

CMD ["./beapin"]
