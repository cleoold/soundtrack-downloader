MAKEFLAGS += --always-make

GO=go
GOOS_ARCH_PAIRS = linux-amd64 linux-arm64 windows-amd64 windows-arm64 darwin-amd64 darwin-arm64
OUTDIR=bin
EXES = downloader meta

build:
	for exe in $(EXES); do $(GO) build -o $(OUTDIR)/$$exe ./cmd/$$exe; done

build-all: $(foreach pair, $(GOOS_ARCH_PAIRS), build-arch-$(pair))

build-arch-%:
	@osarch=$*; \
	os=$$(echo $$osarch | cut -d'-' -f1); \
	arch=$$(echo $$osarch | cut -d'-' -f2); \
	for exe in $(EXES); do \
		if [ "$$os" = "windows" ]; then \
			out="$(OUTDIR)/$$exe-$$os-$$arch.exe"; \
		else \
			out="$(OUTDIR)/$$exe-$$os-$$arch"; \
		fi; \
		CMD="GOOS=$$os GOARCH=$$arch $(GO) build -o $$out ./cmd/$$exe"; \
		echo $$CMD; \
		eval $$CMD; \
	done

test:
	$(GO) test -v ./...

test-cover:
	$(GO) test -v -cover ./...

clean:
	rm -f $(OUTDIR)/*
