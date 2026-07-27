BINARY := relai
DIST   := dist

.PHONY: test
test:
	go vet ./...
	go test ./... -race

.PHONY: build
build:
	go build -o $(BINARY) .

# El target darwin solo enlaza si se ejecuta desde un host macOS: sin fijar
# CGO_ENABLED, Go activa cgo cuando GOOS/GOARCH coinciden con el host, y
# fyne.io/systray necesita cgo para su backend Cocoa. Cruzar a darwin desde
# Linux (o desde macOS con CGO_ENABLED=0) falla — por eso el job `cross` del
# CI compila darwin en un runner macos-latest en vez de cruzar todo desde
# ubuntu-latest. `make cross` en sí mismo solo produce un darwin-arm64 válido
# si lo ejecutas en un Mac.
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
