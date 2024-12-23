build:
	@go build -o bin/scatterfs main.go

run: build
	@./bin/scatterfs

test:
	@go test ./...
