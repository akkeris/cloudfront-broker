#
#

NAME=cloudfront-broker
REPO=akkeris

IMAGE ?= $(NAME)
TAG ?= $(shell git describe --tags --always)
PULL ?= IfNotPresent


SRC=*.go pkg/*/*
SRCDIR=pkg/boker pkg/storage pkg/service
PKG=$(NAME)/pkg/broker $(NAME)/pkg/storage $(NAME)/pkg/service

VERSION=0.1
GIT_COMMIT := $(shell git rev-parse HEAD)
GO_VERSION := $(shell go version | sed 's/^go version go\(\([0-9]*\.[0-9]*\)*\).*$$/\1/')
BUILT := $(shell date +"%F-%I:%M:%S%z")
OSB_VERSION=2.13

BUILD_NO=alpha

BUILD_ARGS=--build-arg VERSION=$(VERSION) --build-arg OSB_VERSION=$(OSB_VERSION) --build-arg BUILD_NO=$(BUILD_NO)

LDFLAGS= -ldflags "-s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.GoVersion=$(GO_VERSION) -X main.Built=$(BUILT) -X main.OSBVersion=$(OSB_VERSION)"

build: $(NAME) ## Builds the cloudfront-broker

$(NAME): $(SRC)
	@echo VERSION="$(VERSION)"
	@echo GIT_COMMIT=$(GIT_COMMIT)
	@echo GO_VERSION="$(GO_VERSION)"
	@echo BUILT=$(BUILT)
	@echo OSB_VERSION=$(OSB_VERSION)
	go build -i $(LDFLAGS) -o $(NAME) .

test: ## Runs the tests
	go get github.com/smartystreets/goconvey
	go test $(shell go list ./... | grep -v /vendor/ | grep -v /test) -logtostderr=1 -stderrthreshold 0

coverage: ## Runs the tests with coverage
	go get github.com/smartystreets/goconvey
	go test -timeout 2400s -coverprofile cover.out -v $(shell go list ./... | grep -v /vendor/ | grep -v /test)  -logtostderr=1 -stderrthreshold 0

linux: $(NAME)-linux ## Builds a Linux executable

$(NAME)-linux: $(SRC) ## Builds a Linux executable
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build $(BUILD_ARGS) -o $(NAME)-linux $(LDFLAGS) $(NAME).go

image:  ## Builds a docker image (AlpineOS)
	docker build $(BUILD_ARGS) -t "$(IMAGE)" .

clean: ## Cleans up build artifacts
	rm -f $(NAME)
	rm -f $(NAME)-linux
	rm -f cover.out

tag: image
	docker tag $(IMAGE) $(REPO)/$(IMAGE):$(TAG)

push: image ## Pushes the image
	docker push $(REPO)/$(IMAGE):$(TAG)
	docker push $(REPO)/$(IMAGE)

lint: $(SRC)
	golint $(NAME).go
	golint pkg/...

tidy: $(SRC)
	go mod tidy

help: ## Shows the help
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
        awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

.PHONY: build test docker linux image clean push deploy-helm help
