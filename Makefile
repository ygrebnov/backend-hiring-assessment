.PHONY: test
test:
	@echo "Running tests..."
	@go clean -testcache
	@go test ./... -count=1 -timeout=600s