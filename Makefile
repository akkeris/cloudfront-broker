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

build: $(NAME) ## Builds the cloudfront-broker

$(NAME): $(SRC)
	go build -i  -o $(NAME) .

test: ## Runs the tests
	go get github.com/smartystreets/goconvey
	go test $(shell go list ./... | grep -v /vendor/ | grep -v /test) -logtostderr=1 -stderrthreshold 0

coverage: ## Runs the tests with coverage
	go get github.com/smartystreets/goconvey
	go test -timeout 2400s -coverprofile cover.out -v $(shell go list ./... | grep -v /vendor/ | grep -v /test)  -logtostderr=1 -stderrthreshold 0

linux: $(NAME)-linux ## Builds a Linux executable

$(NAME)-linux: $(SRC) ## Builds a Linux executable
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o $(NAME)-linux --ldflags="-s" $(NAME).go

image: linux ## Builds a Linux based docker image
	mv $(NAME)-linux $(NAME)
	docker build -t "$(IMAGE):$(TAG)" .
	rm $(NAME)

clean: ## Cleans up build artifacts
	rm -f $(NAME)
	rm -f $(NAME)-linux
	rm -f cover.out
	#go clean --cache

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

.PHONY: build test linux image clean push deploy-helm help
