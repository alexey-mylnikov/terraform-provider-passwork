default: build

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint: fmt vet

.PHONY: install
install:
	go install .

.PHONY: generate
generate:
	go generate ./...
