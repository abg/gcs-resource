GINKGO := go run github.com/onsi/ginkgo/v2/ginkgo
pkgs   = $(shell go list ./...)

DOCKER_IMAGE_NAME ?= gcs-resource
DOCKER_IMAGE_TAG  ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

default: format build unit-tests

format:
	@echo ">> formatting code"
	@go fmt $(pkgs)

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

vet:
	@echo ">> vetting code"
	@go vet $(pkgs)

build:
	@echo ">> building binaries"
	@go build -o assets/in ./cmd/in
	@go build -o assets/out ./cmd/out
	@go build -o assets/check ./cmd/check

unit-tests: deps
	@echo ">> running unit tests"
	@$(GINKGO) version
	@$(GINKGO) -r -race -p -skip-package=integration,vendor

integration-tests: deps
	@echo ">> running integration tests"
	@$(GINKGO) version
	@$(GINKGO) -r integration

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

.PHONY: default deps format style vet build unit-tests integration-tests docker
