.PHONY: check fmt lint test vuln shell build snapshot clean

GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

check: fmt lint test vuln shell

fmt:
	@test -z "$$(gofumpt -l .)" || { gofumpt -l .; echo "run: gofumpt -w ."; exit 1; }

lint:
	GOOS=darwin golangci-lint run ./...
	GOOS=linux golangci-lint run ./...
	GOOS=windows golangci-lint run ./...

test:
	go test -race -count=1 ./...

vuln:
	govulncheck ./...

shell:
	shellcheck install.sh

build:
	go build -trimpath -o bin/claude-switch ./cmd/claude-switch

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
