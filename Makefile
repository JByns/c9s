VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PREFIX  ?= /usr/local

.PHONY: build install mock dev linux test clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/c9s .

install: build
	install -m 0755 bin/c9s $(PREFIX)/bin/c9s

mock:
	go run ./cmd/mockcmon

# dev runs c9s against the mock controller (start `make mock` first).
dev: build
	C9S_CONTROLLER=http://127.0.0.1:9501 CC_PASSWORD=x ./bin/c9s --user admin --write

linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/c9s-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/c9s-linux-arm64 .

test:
	go vet ./...
	go test ./...

clean:
	rm -rf bin dist
