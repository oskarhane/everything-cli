.PHONY: build test lint fmt-check fmt

build:
	go build -ldflags "-X github.com/oskarhane/google-cli/internal/app.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/google-cli .

test:
	go test ./...

lint:
	golangci-lint run

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

fmt:
	gofmt -w .
