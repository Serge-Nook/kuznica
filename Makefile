BINARY := kuznica
BUILD_DIR := build
PREFIX ?= /usr/local

.PHONY: all build run test lint fmt clean install

all: build

build:
	mkdir -p $(BUILD_DIR)
	go build -trimpath -o $(BUILD_DIR)/$(BINARY) ./cmd/kuznica

run: build
	$(BUILD_DIR)/$(BINARY)

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l . )" || { echo "gofmt required:"; gofmt -l .; exit 1; }

fmt:
	gofmt -w .

install: build
	install -Dm755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	install -Dm644 packaging/kuznica.desktop $(DESTDIR)$(PREFIX)/share/applications/kuznica.desktop
	install -Dm644 assets/icons/kuznica.svg $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/kuznica.svg

clean:
	rm -rf $(BUILD_DIR)
