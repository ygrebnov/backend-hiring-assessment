E2E_PROJECT := ads-e2e
E2E_API_PORT := 18080
E2E_DB_PORT := 15432
E2E_USERS_PORT := 18081

E2E_ENV := \
	ADS_API_PORT=$(E2E_API_PORT) \
	ADS_DB_PORT=$(E2E_DB_PORT) \
	USERS_API_PORT=$(E2E_USERS_PORT) \
	AD_PUBLICATION_LIMIT=5 \
	AD_PUBLICATION_WINDOW=10s

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@go test ./... -count=1 -timeout=600s

.PHONY: test-unit-race
test-unit-race:
	@echo "Running unit tests with race detector..."
	@go test ./... -race -count=1 -timeout=600s

.PHONY: mount
mount:
	@echo "Mounting E2E environment..."
	@$(E2E_ENV) docker compose -p $(E2E_PROJECT) down -v --remove-orphans
	@$(E2E_ENV) docker compose -p $(E2E_PROJECT) up -d --build --wait

.PHONY: unmount
unmount:
	@echo "Unmounting E2E environment..."
	@$(E2E_ENV) docker compose -p $(E2E_PROJECT) down -v --remove-orphans

.PHONY: test-e2e
test-e2e:
	@echo "Running end-to-end tests..."
	@ADS_API_URL=http://localhost:$(E2E_API_PORT) \
	 USERS_API_URL=http://localhost:$(E2E_USERS_PORT) \
		go test -race -tags=e2e -count=1 -v -timeout=600s ./ads/internal/cmd/ads-api/e2e/...

.PHONY: test
test: test-unit-race
	@$(MAKE) mount || exit $$?; \
	$(MAKE) test-e2e; \
	status=$$?; \
	$(MAKE) unmount || { \
		if [ $$status -eq 0 ]; then status=1; fi; \
	}; \
	exit $$status
	@echo "All tests passed successfully."