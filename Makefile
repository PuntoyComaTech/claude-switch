.PHONY: check fmt lint test vuln shell build snapshot clean

# Tools installed with `go install` / the golangci-lint script live here.
# Full paths: macOS make (3.81) ignores an exported PATH for simple recipes.
GOBIN := $(shell go env GOPATH)/bin
tool = $(firstword $(wildcard $(GOBIN)/$(1)) $(1))
GOFUMPT := $(call tool,gofumpt)
GOLANGCI := $(call tool,golangci-lint)
GOVULN := $(call tool,govulncheck)

check: fmt lint test vuln shell

fmt:
	@test -z "$$($(GOFUMPT) -l .)" || { $(GOFUMPT) -l .; echo "run: $(GOFUMPT) -w ."; exit 1; }

lint:
	GOOS=darwin $(GOLANGCI) run ./...
	GOOS=linux $(GOLANGCI) run ./...
	GOOS=windows $(GOLANGCI) run ./...

test:
	go test -race -count=1 ./...

vuln:
	$(GOVULN) ./...

shell:
	shellcheck install.sh

build:
	go build -trimpath -o bin/claude-switch ./cmd/claude-switch

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
