VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test clean release install

build:
	go build $(LDFLAGS) -o bin/egafetch ./cmd/egafetch

test:
	go test -race -v ./...

clean:
	rm -rf bin/

release: clean
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/egafetch-linux-amd64 ./cmd/egafetch
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/egafetch-linux-arm64 ./cmd/egafetch
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/egafetch-darwin-amd64 ./cmd/egafetch
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/egafetch-darwin-arm64 ./cmd/egafetch
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/egafetch-windows-amd64.exe ./cmd/egafetch

install:
	go install $(LDFLAGS) ./cmd/egafetch
