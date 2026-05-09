FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o fastkv ./cmd/fastkv

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/fastkv .

EXPOSE 6380

CMD ["./fastkv", "-addr", ":6380"]