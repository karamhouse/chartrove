.ONESHELL: # Applies to every targets in the file! .ONESHELL instructs make to invoke a single instance of the shell and provide it with the entire recipe, regardless of how many lines it contains.
.SHELLFLAGS = -ec
YARN ?= npm exec --yes --package=corepack@0.35.0 -- corepack yarn
NG = node node_modules/@angular/cli/bin/ng.js

########################################################################################################################
# General
########################################################################################################################
.PHONY: test
test: test-backend test-frontend

.PHONY: build-storybook
build-storybook: dep-frontend
	cd frontend && $(NG) run fastenhealth:build-storybook

.PHONY: serve-storybook
serve-storybook: dep-frontend
	cd frontend && $(NG) run fastenhealth:storybook

.PHONY: serve-frontend
serve-frontend: dep-frontend
	cd frontend && $(NG) serve --hmr --live-reload -c dev

.PHONY: serve-frontend-prod
serve-frontend-prod: dep-frontend
	cd frontend && $(NG) serve --configuration prod

.PHONY: serve-backend
serve-backend: dep-backend
	go run backend/cmd/fasten/fasten.go start --config ./config.dev.yaml --debug

.PHONY: migrate
migrate: dep-backend
	go run backend/cmd/fasten/fasten.go migrate --config ./config.dev.yaml --debug

.PHONY: serve-docker-prod
serve-docker-prod:
	docker compose -f docker-compose-prod.yml up -d

.PHONY: serve-docker
serve-docker:
	docker compose up -d


########################################################################################################################
# Backend
########################################################################################################################

.PHONY: clean-backend
clean-backend:
	go clean

.PHONY: dep-backend
dep-backend:
	go mod tidy && go mod vendor
	cd scripts && go generate ./...

.PHONY: prepare-backend
prepare-backend:
	go generate ./scripts/related_versions


.PHONY: test-backend
test-backend: prepare-backend
	go vet ./...
	go test -v ./...

.PHONY: test-backend-coverage
test-backend-coverage: prepare-backend
	go test -coverprofile=backend-coverage.txt -covermode=atomic -v ./...

.PHONY: generate-backend
generate-backend:
	go generate ./...
	tygo generate

########################################################################################################################
# Frontend
########################################################################################################################
.PHONY: dep-frontend
dep-frontend:
	cd frontend && $(YARN) install --frozen-lockfile --ignore-optional --registry https://registry.npmjs.org --network-concurrency 4

.PHONY: build-frontend-sandbox
build-frontend-sandbox: dep-frontend
	cd frontend && $(NG) build --configuration sandbox --output-path=../dist

.PHONY: build-frontend-prod
build-frontend-prod: dep-frontend
	cd frontend && $(NG) build --configuration prod --output-path=../dist

.PHONY: build-frontend-desktop-sandbox
build-frontend-desktop-sandbox: dep-frontend
	cd frontend && $(NG) build --configuration desktop_sandbox --output-path=../dist

.PHONY: build-frontend-desktop-prod
build-frontend-desktop-prod: dep-frontend
	cd frontend && $(NG) build --configuration desktop_prod --output-path=../dist

.PHONY: build-frontend-offline-sandbox
build-frontend-offline-sandbox: dep-frontend
	cd frontend && $(NG) build --configuration offline_sandbox --output-path=../dist

.PHONY: test-frontend
# reduce logging, disable angular-cli analytics for ci environment
test-frontend: dep-frontend
	cd frontend && $(NG) test --watch=false

.PHONY: test-frontend-coverage
# reduce logging, disable angular-cli analytics for ci environment
test-frontend-coverage: dep-frontend
	cd frontend && $(NG) test --watch=false --code-coverage

.PHONY: test-frontend-coverage-ci
# reduce logging, disable angular-cli analytics for ci environment
test-frontend-coverage-ci: dep-frontend
	cd frontend && npx ng test --watch=false --code-coverage --browsers=ChromeHeadlessCI
