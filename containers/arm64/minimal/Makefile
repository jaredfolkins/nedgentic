# Local image name. Dev builds of ned default to ned-minimal:dev; release
# builds (make release at the repo root) tag ned-minimal:<version>.
IMAGE_NAME := ned-minimal
TAG ?= dev
BUILD_FLAGS ?=

.PHONY: build update

build:
	container build $(BUILD_FLAGS) -t $(IMAGE_NAME):$(TAG) .

update: BUILD_FLAGS += --pull
update: build
