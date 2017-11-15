GO = go
GOFMT = gofmt
GOLINT = golint

GO_PACKAGES = ./cmd/... ./pkg/...

all: controller

generate:
	$(GO) generate $(GO_PACKAGES)

controller:
	$(GO) build -o $@ ./cmd/controller

controller-static:
	CGO_ENABLED=0 $(GO) build -installsuffix cgo -o $@ ./cmd/controller

test:
	$(GO) test $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_PACKAGES)

lint:
	@set -e; for p in $(GO_PACKAGES); do \
	  echo $(GOLINT) $$p; \
	  $(GOLINT) $$p; \
	done

fmt:
	$(GOFMT) -s -w $(shell $(GO) list -f '{{.Dir}}' $(GO_PACKAGES))

.PHONY: all test clean vet fmt
