# Makefile — Metrics
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.metricsVersion=$(VERSION)
BINDIR  := $(HOME)/bin

.PHONY: all build clean

all: build

build:
	@echo "  → metrics $(VERSION)"
	@CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/metrics ./cmd/metrics/

clean:
	@rm -f $(BINDIR)/metrics
