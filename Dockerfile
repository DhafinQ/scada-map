# Ganti dari golang:1.22-alpine menjadi golang:1.24-alpine
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server main.go

FROM alpine:3.20

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Jakarta

COPY --from=builder /app/server .
COPY index.html config.html ./

EXPOSE 8080
CMD ["./server"]