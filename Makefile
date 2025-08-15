# Makefile - build + package helpers for linux amd64 and arm64 (lambda extension zips)
BINARY := nats-extension
SRC := main.go
LDFLAGS := -s -w
CGO_ENABLED := 0
DIST := dist

.PHONY: all build-amd64 build-arm64 build-all package-amd64 package-arm64 package-all release-amd64 release-arm64 clean

all: build-all

build-amd64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-amd64 $(SRC)

build-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-arm64 $(SRC)

build-all: build-amd64 build-arm64

# Package a platform-specific zip suitable for an external Lambda extension.
# Each zip places the executable under extensions/ so that when AWS extracts into /opt
# the binary ends up at /opt/extensions/nats-extension
package-amd64: build-amd64
	mkdir -p $(DIST)/extensions
	cp $(BINARY)-linux-amd64 $(DIST)/extensions/$(BINARY)
	chmod +x $(DIST)/extensions/$(BINARY)
	( cd $(DIST) && zip -q $(BINARY)-linux-amd64.zip extensions/$(BINARY) && rm -rf extensions )

package-arm64: build-arm64
	mkdir -p $(DIST)/extensions
	cp $(BINARY)-linux-arm64 $(DIST)/extensions/$(BINARY)
	chmod +x $(DIST)/extensions/$(BINARY)
	( cd $(DIST) && zip -q $(BINARY)-linux-arm64.zip extensions/$(BINARY) && rm -rf extensions )

package-all: package-amd64 package-arm64

# Build single binary named 'nats-extension' for amd64 (overwrites BINARY)
release-amd64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(SRC)

# Build single binary named 'nats-extension' for arm64 (overwrites BINARY)
release-arm64:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(SRC)

clean:
	rm -f $(BINARY)-linux-* $(BINARY)
	rm -rf $(DIST)