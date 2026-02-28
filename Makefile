GO := /usr/local/go/bin/go
BINARY := smart-trunc
VERSION := 0.2.0
LDFLAGS := -s -w -X main.version=$(VERSION)

DIST := dist
PLATFORMS := linux/amd64 darwin/amd64 darwin/arm64

.PHONY: build test test-update clean install demo release

build:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	$(GO) test ./... -v -count=1

test-update:
	UPDATE_GOLDEN=1 $(GO) test ./... -v -count=1

release:
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(DIST)/$(BINARY)-$${os}-$${arch}; \
		echo "Building $${output}..."; \
		CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $${output} . ; \
	done
	@echo "Done. Binaries in $(DIST)/"
	@ls -lh $(DIST)/

clean:
	rm -f $(BINARY) coverage.out
	rm -rf $(DIST)

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)

demo: build
	@echo "=== short input (passthrough) ==="
	echo "hello world" | ./$(BINARY)
	@echo ""
	@echo "=== pytest failure sample ==="
	cat testdata/pytest_failure/input.txt 2>/dev/null | ./$(BINARY) --mode test --limit 500 || echo "(no test data yet)"

coverage:
	$(GO) test ./... -coverprofile=coverage.out -count=1
	$(GO) tool cover -func=coverage.out
