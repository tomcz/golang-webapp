GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X github.com/tomcz/golang-webapp/build.commit=${GITCOMMIT}

.PHONY: all
all: clean format lint build-prod

.PHONY: clean
clean:
	rm -rf target

target:
	mkdir target

.PHONY: format
format:
ifeq (, $(shell which goimports))
	go install golang.org/x/tools/cmd/goimports@latest
endif
	goimports -w -local github.com/tomcz/golang-webapp .

.PHONY: lint
lint:
ifeq (, $(shell which staticcheck))
	go install honnef.co/go/tools/cmd/staticcheck@latest
endif
	staticcheck ./...

.PHONY: build-dev
build-dev: target
	go build -ldflags "${LDFLAGS}" -o target/golang-webapp ./cmd/webapp/...

.PHONY: build-prod
build-prod: target
	go build -tags prod -ldflags "${LDFLAGS}" -o target/golang-webapp ./cmd/webapp/...

.PHONY: run-dev
run-dev: build-dev
	./target/golang-webapp

.PHONY: run-prod
run-prod: build-prod
	./target/golang-webapp
