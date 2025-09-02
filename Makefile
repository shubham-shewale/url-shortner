.PHONY: build test test-race clean

build:
	go build ./cmd/api
	go build ./cmd/redirect

test:
	go test ./... -v

test-race:
	go test ./... -race -v

clean:
	go clean
	rm -f api redirect

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html