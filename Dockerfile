FROM golang:1.23-alpine as builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o main main.go

FROM alpine:latest

RUN apk --no-cache add sqlite-libs

WORKDIR /root/

COPY --from=builder /app/main .

CMD ["./main"]
