HOSTNAME=registry.terraform.io
NAMESPACE=favoretti
NAME=firestore
BINARY=terraform-provider-${NAME}
VERSION=$(shell git describe --tags --always --dirty)
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

default: build

.PHONY: build
build:
	go build -o ${BINARY}

.PHONY: install
install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}/

.PHONY: test
test:
	go test -v -cover ./...

.PHONY: testacc
testacc:
	TF_ACC=1 go test -v -cover ./... -timeout 120m

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	go fmt ./...
	gofmt -s -w .

.PHONY: generate
generate:
	go generate ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f ${BINARY}
	rm -rf dist/

.PHONY: all
all: fmt lint test build
