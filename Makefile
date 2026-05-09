APP_NAME=fastkv

.PHONY: run build test fmt clean docker-build docker-run

run:
	go run ./cmd/fastkv

build:
	go build -o bin/$(APP_NAME) ./cmd/fastkv

test:
	go test ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin

docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run --rm -p 6380:6380 $(APP_NAME)