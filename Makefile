.PHONY: verify verify-repeat

verify:
	version="$$(go env GOVERSION)"; test "$${version%%-*}" = "go1.26.6"
	test -z "$$(gofmt -l $$(git ls-files --cached --others --exclude-standard -- '*.go'))"
	go vet -mod=readonly ./...
	go test -mod=readonly -race ./...
	go test -mod=readonly -coverprofile=coverage.out ./pkg/scim
	coverage="$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/, "", $$3); print $$3}')"; awk -v coverage="$$coverage" 'BEGIN {exit !(coverage >= 90)}'

verify-repeat:
	go test -mod=readonly -race -count=50 ./...
