# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: scheduler

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Build manager binary
scheduler: mod format
	go build -o bin/scheduler main.go

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

image: scheduler
	docker build -t hexilee/naglfar-scheduler .

deploy: image
	kubectl apply -f deploy/naglfar-scheduler.yaml

upgrade: deploy
	kubectl rollout restart deployment/naglfar-scheduler -n kube-system

destroy:
	kubectl delete -f deploy/naglfar-scheduler.yaml

describe:
	kubectl describe deployment/naglfar-scheduler -n kube-system

log:
	kubectl logs -f deployment/naglfar-scheduler -n kube-system