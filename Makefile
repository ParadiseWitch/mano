BIN := dist/orgmaid
WIN := dist/orgmaid-windows-amd64.exe
# 本地构建从 git 取版本：正好在 tag 上就是 tag 名，tag 后有提交就带后缀；
# 拿不到 git（比如在发布包里构建）退回 devel。
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo devel)
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
