# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CONTROLLER_GEN=$(GOBIN)/controller-gen

all: scheduler

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Build manager binary
scheduler: mod format
	go build -o bin/scheduler main.go

# Build webhook manager binary
webhook: mod format
	go build -o bin/webhook webhook/main.go

clean:
	rm -rf bin

format: vet fmt

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

mod:
	@echo "go mod tidy"
	GO111MODULE=on go mod tidy
	@git diff --exit-code -- go.sum go.mod

# Install CRDs into a cluster
install: manifests
	kustomize build deploy/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build deploy/crd | kubectl delete -f -

image:
	DOCKER_BUILDKIT=1 docker build -t naglfar-scheduler -f docker/scheduler/Dockerfile .

webhook-image: docker/manager/Dockerfile **/*.go
	DOCKER_BUILDKIT=1 docker build -t naglfarcloud-manager -f docker/manager/Dockerfile .

upload: image
	minikube image load naglfar-scheduler

upload-webhook: webhook-image
	minikube image load naglfarcloud-manager

deploy: install upload deploy/naglfar-scheduler.yaml
	kubectl apply -f deploy/naglfar-scheduler.yaml

deploy-manager: upload-webhook deploy/webhook/*.yaml
	kubectl apply -f deploy/webhook/namespace.yaml
	kubectl apply -f deploy/webhook/manager.yaml

deploy-cert: deploy-manager
	kubectl apply -f deploy/webhook/cert.yaml

deploy-webhook: deploy-cert
	kubectl apply -f deploy/webhook/webhook.yaml

upgrade: deploy
	kubectl rollout restart deployment/naglfar-scheduler -n kube-system

upgrade-webhook: deploy-webhook
	kubectl rollout restart deployment/naglfar-labeler -n naglfar-system

destroy:
	kubectl delete -f deploy/naglfar-scheduler.yaml

destroy-webhook:
	kubectl delete -f deploy/webhook/webhook.yaml
	kubectl delete -f deploy/webhook/cert.yaml
	kubectl delete -f deploy/webhook/manager.yaml
	kubectl delete -f deploy/webhook/namespace.yaml

describe:
	kubectl describe deployment/naglfar-scheduler -n kube-system

log:
	kubectl logs -f deployment/naglfar-scheduler -n kube-system

log-webhook:
	kubectl logs -f deployment/naglfar-labeler -n naglfar-system -c manager

manifests: pkg/api/v1/*.go
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./pkg/..." output:crd:artifacts:config=deploy/crd/bases

# Generate code
generate: manifests
        $(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/..."

install-controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
endif

install-kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_TMP_DIR ;\
	wget https://github.com/kubernetes-sigs/kustomize/archive/kustomize/v3.8.8.tar.gz; \
	tar xvf v3.8.8.tar.gz; \
	cd kustomize-kustomize-v3.8.8/kustomize/; \
	go install; \
	rm -rf $$KUSTOMIZE_TMP_DIR ;\
	}
endif

install-cert-manager:
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.2.0/cert-manager.yaml
