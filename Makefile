BINARY := relai
DIST   := dist

.PHONY: test
test:
	go vet ./...
	go test ./... -race

.PHONY: build
build:
	go build -o $(BINARY) .

.PHONY: cross
cross:
	mkdir -p $(DIST)
	GOOS=darwin  GOARCH=arm64 go build -o $(DIST)/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o $(DIST)/$(BINARY)-windows-amd64.exe .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(DIST)/$(BINARY)-linux-amd64 .
	@ls -la $(DIST)

.PHONY: clean
clean:
	rm -rf $(DIST) $(BINARY)
