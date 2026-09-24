default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

# Everything, including the tests that drive Terraform against a local mesh.
# They start the backend themselves and skip when no mesh checkout is nearby.
test:
	go test -timeout=30m -parallel=8 ./...

# The same acceptance tests against mesh-enterprise: Postgres, Valkey and an
# SQS from its docker-compose.yml, on ports of their own, torn down after.
testenterprise:
	PARTITE_TEST_BACKENDS=enterprise go test -timeout=45m -parallel=4 ./...

# Both backends. This is what catches a provider that works against one and
# not the other.
testall:
	PARTITE_TEST_BACKENDS=all go test -timeout=60m -parallel=4 ./...

# Just the tests that need no backend and no Terraform binary. Sub-second, so
# this is the one to run while editing schemas.
testunit:
	go test -timeout=60s -run 'TestProviderSchemaSnapshot|TestSchemaMatchesModel|TestEveryResourceHasADataSource|TestIDIsComputed|TestSetFields' ./internal/provider/

# Accept an intended schema change.
updategolden:
	go test ./internal/provider -run TestProviderSchemaSnapshot -update

testacc: test

.PHONY: fmt lint test testunit testenterprise testall testacc updategolden build install generate
