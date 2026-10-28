BIN := bin/bluehandle

.PHONY: all test vet build clean

all: vet test build

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o $(BIN) ./cmd/bluehandle

clean:
	rm -rf bin dist
