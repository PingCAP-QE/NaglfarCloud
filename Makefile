IMG_PREFIX ?=
SCHEDULER_TAG ?= v1.19.8
TAG ?= latest
IMG_SCHEDULER ?= ${IMG_PREFIX}naglfar-scheduler:${SCHEDULER_TAG}
IMG_MANAGER ?= ${IMG_PREFIX}naglfarcloud-manager:${TAG}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CONTROLLER_GEN=$(GOBIN)/controller-gen

all: manifests scheduler manager

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Build scheduler binary
scheduler: mod format
	go build -o bin/scheduler main.go

# Build webhook manager binary
manager: mod format
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

manifests:
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./pkg/..." output:crd:artifacts:config=deploy/crd/bases
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/..."

# Install CRDs into a cluster
install-manifests: manifests
	kustomize build deploy/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall-manifests: manifests
	kustomize build deploy/crd | kubectl delete -f -

scheduler-image:
	DOCKER_BUILDKIT=1 docker build -t ${IMG_SCHEDULER} -f docker/scheduler/Dockerfile .

manager-image:
	DOCKER_BUILDKIT=1 docker build -t ${IMG_MANAGER} -f docker/manager/Dockerfile .

upload-image: scheduler-image manager-image
	docker push ${IMG_SCHEDULER}
	docker push ${IMG_MANAGER}

# deploy-scheduler: install-manifests
# 	kubectl apply -f deploy/naglfar-scheduler.yaml

deploy-manager: install-manifests
	kubectl apply -f deploy/webhook/namespace.yaml
	kubectl apply -f deploy/webhook/cert.yaml
	kubectl apply -f deploy/webhook/manager.yaml
	kubectl rollout status deployments/naglfar-labeler -n naglfar-system
	kubectl apply -f deploy/webhook/webhook.yaml

# upgrade-scheduler: deploy-scheduler
# 	kubectl rollout restart deployment/naglfar-scheduler -n kube-system
# 	kubectl rollout status deployment/naglfar-scheduler -n kube-system

upgrade-manager: deploy-manager
	# kubectl delete -f deploy/webhook/webhook.yaml
	kubectl rollout restart deployment/naglfar-labeler -n naglfar-system
	kubectl rollout status deployment/naglfar-labeler -n naglfar-system
	# kubectl apply -f deploy/webhook/webhook.yaml

# destroy-scheduler:
# 	kubectl delete -f deploy/naglfar-scheduler.yaml

destroy-manager:
	kubectl delete -f deploy/webhook/webhook.yaml
	kubectl delete -f deploy/webhook/manager.yaml
	kubectl delete -f deploy/webhook/cert.yaml
	kubectl delete -f deploy/webhook/namespace.yaml

# describe-scheduler:
# 	kubectl describe deployment/naglfar-scheduler -n kube-system

describe-manager:
	kubectl describe deployment/naglfar-labeler -n naglfar-system

# log-scheduler:
# 	kubectl logs -f deployment/naglfar-scheduler -n kube-system

log-manager:
	kubectl logs -f deployment/naglfar-labeler -n naglfar-system -c manager

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
