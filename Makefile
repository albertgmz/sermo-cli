SERVER ?= https://fcuba.net
VERSION ?= dev
LDFLAGS := -s -w -X github.com/albertgmz/sermo-cli/internal/buildinfo.Version=$(VERSION)

.PHONY: build test vet lint generate spec

# Build the sermo binary.
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o sermo ./cmd/sermo

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

# Make the typed client and the SpecVersion constant again from api/openapi.json.
generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.json
	v=$$(grep -o '"info":{[^}]*' api/openapi.json | grep -o '"version":"[^"]*"' | cut -d'"' -f4); \
	printf 'package api\n\n// SpecVersion is the info.version of api/openapi.json.\nconst SpecVersion = "%s"\n' "$$v" > internal/api/specversion.go

# Download the current spec from SERVER, then make the typed client again.
spec:
	curl -fsSL "$(SERVER)/api/v1/openapi.json" -o api/openapi.json
	$(MAKE) generate
