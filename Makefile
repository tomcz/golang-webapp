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
ifneq ($(shell which npx),)
	npx prettier --print-width 120 --write "static/*.(js|css)"
endif

.PHONY: lint
lint:
	golangci-lint run --timeout 10m

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: test
test:
	go test ./...

.PHONY: build-dev
build-dev: target
	go build -tags dev -ldflags "${LDFLAGS}" -o target ./cmd/...

.PHONY: build-prod
build-prod: target
	go build -tags prod -ldflags "${LDFLAGS}" -o target ./cmd...

.PHONY: dev
dev: build-dev
	./target/webapp

.PHONY: run
run: build-prod
	./target/webapp

.PHONY: keygen
keygen:
ifeq ($(shell which pwgen),)
	$(error "Please install pwgen (e.g. brew install pwgen)")
else
	@echo "SESSION_AUTH_KEY='$(shell pwgen 64 1)'"
	@echo "SESSION_ENC_KEY='$(shell pwgen 64 1)'"
endif
