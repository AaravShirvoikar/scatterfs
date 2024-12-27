build:
	@go build -o bin/scatterfs main.go

run: build
	@./bin/scatterfs

test:
	@go test ./...

clean:
	@rm -rf file_storage encryption_keys
