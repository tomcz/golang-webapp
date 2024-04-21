GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X github.com/tomcz/golang-webapp/build.commit=${GITCOMMIT}

.PHONY: all
all: clean format lint test build-prod

.PHONY: clean
clean:
	rm -rf target

target:
	mkdir target

.PHONY: format
format:
	goimports -w -local github.com/tomcz/golang-webapp .

.PHONY: lint
lint:
	golangci-lint run

.PHONY: tidy
tidy:
	go mod tidy -compat=1.20

.PHONY: test
test:
	go test ./...

.PHONY: keygen
keygen:
	go run ./cmd/keygen/main.go

.PHONY: build-dev
build-dev: target
	go build -tags dev -ldflags "${LDFLAGS}" -o target ./cmd/...

.PHONY: build-prod
build-prod: target
	go build -tags prod -ldflags "${LDFLAGS}" -o target ./cmd...

.PHONY: dev
dev: build-dev
	./target/webapp

.PHONY: prod
prod: build-prod
	./target/webapp
