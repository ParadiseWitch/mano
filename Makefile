BIN := dist/mano
WIN := dist/mano-windows-amd64.exe
VERSION ?= devel
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build build-windows dist run test fmt vet tidy clean

all: build

build:
	go build $(LDFLAGS) -o $(BIN) .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(WIN) .

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
