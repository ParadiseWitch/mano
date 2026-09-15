BIN := dist/mano
WIN := dist/mano-windows-amd64.exe

.PHONY: all build build-windows dist run test fmt vet tidy clean

all: build

build:
	go build -o $(BIN) .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(WIN) .

dist: build build-windows

run: build
	./$(BIN)

run-windows: build-windows
	./$(WIN)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf dist
