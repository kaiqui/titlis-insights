.PHONY: run build lint test tidy

run:
	INSIGHTS_INTERNAL_SECRET=dev-secret \
	INSIGHTS_USE_STUB_SOURCE=true \
	go run ./cmd/insights

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/insights ./cmd/insights

lint:
	go vet ./...

test:
	go test ./... -count=1 -race

tidy:
	go mod tidy
