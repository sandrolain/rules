.PHONY: proto
proto:
	./build-proto.sh

.PHONY: test-coverage
test-coverage:
	./test-coverage.sh

.PHONY: test-e2e
test-e2e:
	go test ./e2e -v

.PHONY: test-all
test-all: test-coverage test-e2e