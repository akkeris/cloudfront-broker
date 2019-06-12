#
#

NAME=cloudfront-broker

IMAGE ?= akkeris/$(NAME)
TAG ?= $(shell git describe --tags --always)
PULL ?= IfNotPresent

SRC=*.go pkg/*/*.go
PKG=$(NAME)/pkg/broker $(NAME)/pkg/storage $(NAME)/pkg/service

build: $(NAME) ## Builds the cloudfront-broker

$(NAME): $(SRC) ## Builds the cloudfront-broker
	go build -i  -o $(NAME) .

test: ## Runs the tests
	go test $(PKG)

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
	go clean --cache


push: image ## Pushes the image
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE)

deploy-helm: image ## Deploys image with helm
	helm upgrade --install broker-skeleton --namespace broker-skeleton \
	charts/$(NAME) \
	--set image="$(IMAGE):$(TAG)",imagePullPolicy="$(PULL)"

lint: $(SRC)
	golint $(SRC)

vet: $(SRC)
	go vet $(NAME)/pkg/...

help: ## Shows the help
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
        awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

.PHONY: build test linux image clean push deploy-helm help
