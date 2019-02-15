# If the USE_SUDO_FOR_DOCKER env var is set, prefix docker commands with 'sudo'
ifdef USE_SUDO_FOR_DOCKER
	SUDO_CMD = sudo
endif

NAME=cloudfront-broker

IMAGE ?= quay.octanner.io/cloudops/$(NAME)
TAG ?= $(shell git describe --tags --always)
PULL ?= IfNotPresent


build: ## Builds the starter pack
	go build -i  -o $(NAME) cloudfront-broker/cmd/servicebroker

test: ## Runs the tests
	go test -v $(shell go list ./... | grep -v /vendor/ | grep -v /test/)

linux: ## Builds a Linux executable
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o $(NAME)-linux --ldflags="-s" $(NAME)/cmd/servicebroker

image: linux ## Builds a Linux based image
	cp $(NAME)-linux image/$(NAME)
	$(SUDO_CMD) docker build image/ -t "$(IMAGE):$(TAG)"

clean: ## Cleans up build artifacts
	rm -f $(NAME)
	rm -f $(NAME)-linux
	rm -f servicebroker
	rm -f image/$(NAME)

push: image ## Pushes the image
	$(SUDO_CMD) docker push "$(IMAGE):$(TAG)"

deploy-helm: image ## Deploys image with helm
	helm upgrade --install broker-skeleton --namespace broker-skeleton \
	charts/$(NAME) \
	--set image="$(IMAGE):$(TAG)",imagePullPolicy="$(PULL)"

deploy-openshift: image ## Deploys image to openshift
	oc project osb-starter-pack || oc new-project osb-starter-pack
	openshift/deploy.sh $(IMAGE):$(TAG)

create-ns: ## Cleans up the namespaces
	kubectl create ns test-ns

provision: create-ns ## Provisions a service instance
	kubectl apply -f manifests/service-instance.yaml

bind: ## Creates a binding
	kubectl apply -f manifests/service-binding.yaml

help: ## Shows the help
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
        awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

.PHONY: build test linux image clean push deploy-helm deploy-openshift create-ns provision bind help
