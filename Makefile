.PHONY: build test test-pg vet clean

## Verify the module compiles cleanly.
build:
	go build ./...

## Unit tests (sqlite-backed, no DB required).
test:
	go test ./...

## Integration tests against a real Postgres instance.
## Expects POSTGRES_URL; the driver-specific provisioning is only covered here.
test-pg:
	go test -tags=integration ./...

vet:
	go vet ./...

clean:
	rm -rf bin/
