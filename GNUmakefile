BINARY := terraform-provider-laravelforge

default: build

build:
	go build -o $(BINARY)

install:
	go install

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint:
	golangci-lint run

generate:
	go generate ./...

# Generate docs/ from the provider schema (build + schema export + tfplugindocs).
docs:
	bash scripts/gen-docs.sh

test:
	go test ./... -timeout 120s

# Acceptance tests hit the real Forge API; requires FORGE_TOKEN.
testacc:
	TF_ACC=1 go test ./... -v -timeout 120m

.PHONY: default build install tidy fmt vet lint generate docs test testacc
