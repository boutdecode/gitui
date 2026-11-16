.PHONY: run run-fake test vet build
run:
	go run ./cmd/gitui
run-fake:
	go run ./cmd/gitui --fake
test:
	go test ./...
vet:
	go vet ./...
build:
	go build -o gitui ./cmd/gitui
