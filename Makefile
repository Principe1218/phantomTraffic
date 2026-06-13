# PhantomTraffic build & quality gates.
# Tool versions are pinned here (not in go.mod) so they add no runtime deps.

GO              ?= go
GOLANGCI_LINT   ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
STATICCHECK     ?= honnef.co/go/tools/cmd/staticcheck@2025.1.1
GOSEC           ?= github.com/securego/gosec/v2/cmd/gosec@v2.21.4
GOVULNCHECK     ?= golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: all build test lint vuln vet staticcheck gosec golangci tidy-check

all: build test lint vuln

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

staticcheck:
	$(GO) run $(STATICCHECK) ./...

gosec:
	$(GO) run $(GOSEC) -quiet ./...

golangci:
	$(GO) run $(GOLANGCI_LINT) run ./...

# lint runs the full static-analysis suite required by AGENTS.md Section 8.1.
lint: vet staticcheck gosec golangci

# vuln runs dependency vulnerability scanning required by AGENTS.md Section 7.4.
vuln:
	$(GO) run $(GOVULNCHECK) ./...

# tidy-check fails CI if go.mod/go.sum are not tidy.
tidy-check:
	$(GO) mod tidy
	git diff --exit-code -- go.mod go.sum
