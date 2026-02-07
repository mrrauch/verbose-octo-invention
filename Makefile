# openstack-operator Makefile

IMG ?= openstack-operator:latest
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
LOCALBIN ?= $(shell pwd)/bin

.PHONY: all
all: generate fmt vet build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: controller-gen ## Generate code (DeepCopy, RBAC, CRDs).
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONTROLLER_GEN) rbac:roleName=openstack-operator-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: generate fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: ## Run golangci-lint.
	golangci-lint run

##@ Build

.PHONY: build
build: generate fmt vet ## Build operator binary.
	go build -o bin/openstack-operator ./cmd/main.go

.PHONY: run
run: generate fmt vet ## Run against the configured cluster.
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build docker image.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push docker image.
	docker push $(IMG)

##@ Deployment

.PHONY: install
install: generate ## Install CRDs into the cluster.
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the cluster.
	kubectl delete -f config/crd/bases/

.PHONY: deploy
deploy: generate ## Deploy operator to the cluster.
	kubectl apply -k config/default/

.PHONY: undeploy
undeploy: ## Undeploy operator from the cluster.
	kubectl delete -k config/default/

##@ Tools

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Install controller-gen.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0

$(LOCALBIN):
	mkdir -p $(LOCALBIN)
