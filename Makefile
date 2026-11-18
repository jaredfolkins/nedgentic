BIN_DIR := bin
NED_PROFILE ?= default
# Apple's container runtime only runs on Apple silicon, so the macOS ned
# binary is always darwin/arm64; ned checks for macOS 26+ at startup.
NED_DARWIN_ARCH := arm64

.PHONY: build build-ned ned release clean

build: build-ned

LDFLAGS := -ldflags "-X main.BuildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"

build-ned:
	mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=$(NED_DARWIN_ARCH) go build $(LDFLAGS) -o $(BIN_DIR)/ned ./cmd/ned
	mkdir -p $(HOME)/go/bin
	rm -f $(HOME)/go/bin/ned
	cp $(BIN_DIR)/ned $(HOME)/go/bin/ned

# make release VERSION=vX.Y.Z: build the minimal image, export it as the
# GitHub release asset, and build a ned that downloads and verifies that asset
# as its default image. Writes dist/ only; publishing is a separate step.
VERSION ?=
RELEASE_REPO := jaredfolkins/nedgentic
DIST_DIR := dist
RELEASE_IMAGE := ned-minimal:$(VERSION)
RELEASE_ASSET := ned-minimal-arm64.oci.tar.gz
RELEASE_NED := ned-darwin-arm64
RELEASE_URL := https://github.com/$(RELEASE_REPO)/releases/download/$(VERSION)/$(RELEASE_ASSET)
RELEASE_PKG := nedgentic/internal/release

release:
	@case "$(VERSION)" in v[0-9]*) ;; *) echo "usage: make release VERSION=vX.Y.Z" >&2; exit 1 ;; esac
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR) $(BIN_DIR)
	$(MAKE) -C containers/arm64/minimal TAG=$(VERSION)
	container image save --platform linux/arm64 --output $(DIST_DIR)/ned-minimal-arm64.oci.tar $(RELEASE_IMAGE)
	gzip -n $(DIST_DIR)/ned-minimal-arm64.oci.tar
	sha=$$(shasum -a 256 $(DIST_DIR)/$(RELEASE_ASSET) | cut -d' ' -f1) && \
	GOOS=darwin GOARCH=$(NED_DARWIN_ARCH) go build -trimpath -ldflags "\
		-X main.BuildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ') \
		-X $(RELEASE_PKG).Version=$(VERSION) \
		-X $(RELEASE_PKG).ImageRef=$(RELEASE_IMAGE) \
		-X $(RELEASE_PKG).ImageURL=$(RELEASE_URL) \
		-X $(RELEASE_PKG).ImageSHA256=$$sha" -o $(BIN_DIR)/ned ./cmd/ned
	cp $(BIN_DIR)/ned $(DIST_DIR)/$(RELEASE_NED)
	cd $(DIST_DIR) && shasum -a 256 $(RELEASE_ASSET) $(RELEASE_NED) > SHA256SUMS
	@echo
	@cat $(DIST_DIR)/SHA256SUMS
	@echo
	@echo "Release files are in $(DIST_DIR)/. Nothing was published. To publish:"
	@echo "  git tag $(VERSION) && git push origin $(VERSION)"
	@echo "  gh release create $(VERSION) --repo $(RELEASE_REPO) --title $(VERSION) --generate-notes $(DIST_DIR)/$(RELEASE_ASSET) $(DIST_DIR)/$(RELEASE_NED) $(DIST_DIR)/SHA256SUMS"

ned: build-ned
	./bin/ned gentic $(NED_PROFILE)

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
