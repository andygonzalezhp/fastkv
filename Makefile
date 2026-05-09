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

bench-get:
	go run ./cmd/bench -mode=get -clients=50 -ops=1000 -keyspace=1000

bench-set:
	go run ./cmd/bench -mode=set -clients=50 -ops=1000 -keyspace=1000

bench-mixed:
	go run ./cmd/bench -mode=mixed -clients=50 -ops=1000 -keyspace=1000

bench-set-fast:
	go run ./cmd/bench -mode=set -clients=50 -ops=1000 -keyspace=1000

run-fast:
	go run ./cmd/fastkv -sync-policy=none