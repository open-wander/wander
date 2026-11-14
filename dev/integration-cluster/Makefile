# -------------------------------------------------------------------------------
# Makefile -- Wander integration cluster: 3 servers, 3 clients, Consul, Vault
#
# Project: Wander
#
# Every target builds and starts what it needs: `make test` brings the cluster
# up, and the cluster runs Wander built from this checkout unless
# WANDER_VERSION names a published release. The choice holds until `make down`.
# Every recipe clears NOMAD_*, WANDER_*, CONSUL_* and VAULT_* first.
# -------------------------------------------------------------------------------

SHELL := /bin/bash

# --- keep the harness's own knobs, drop every other such var ---
CLEAN_ENV := env $(shell env | grep -oE '^(NOMAD|WANDER|CONSUL|VAULT)_[A-Za-z0-9_]*' | grep -vxE '(WANDER|CONSUL|VAULT)_VERSION|WANDER_IMAGE' | sed 's/^/-u /')
COMPOSE := $(CLEAN_ENV) docker compose -f compose.yml

# --- WANDER_IMAGE, else a release, else what the cluster runs now ---
CURRENT_IMAGE := $(shell cat .cache/wander-image 2>/dev/null)
ifneq ($(strip $(WANDER_VERSION)),)
WANDER_IMAGE ?= ghcr.io/open-wander/wander:$(WANDER_VERSION)
endif
WANDER_IMAGE   ?= $(or $(CURRENT_IMAGE),wander-local)
CONSUL_VERSION ?= 2.1.0
VAULT_VERSION  ?= 2.2.2
export WANDER_IMAGE CONSUL_VERSION VAULT_VERSION

# --- source build of the checkout this harness lives in ---
REPO_ROOT  := $(abspath ../..)
WANDER_BIN := $(REPO_ROOT)/pkg/linux_amd64/wander
GO_BIN_DIR := $(or $(shell go env GOBIN 2>/dev/null),$(shell go env GOPATH 2>/dev/null)/bin)
TEMPL      := $(GO_BIN_DIR)/templ
UI_DEPS    := $(REPO_ROOT)/ui/node_modules/.package-lock.json

# --- images the test jobs run, preloaded into each client ---
ENVOY_VERSION ?= 1.39.1
IMAGES := busybox:1.37 gcr.io/google_containers/pause-amd64:3.1 envoyproxy/envoy:v$(ENVOY_VERSION)

NODE_IMAGE := wander-it-node:consul$(CONSUL_VERSION)

ifeq ($(WANDER_IMAGE),wander-local)
UP_DEPS := local-image
endif

.PHONY: help images wander-bin local-image node-image up down test logs ps env

help:
	@printf '%s\n' \
	  'make test [TESTS="a b"]       build, start the cluster, run the suite (or named tests)' \
	  'make up                       build this checkout and start the cluster' \
	  'make up WANDER_VERSION=0.1.1  start on a published release instead' \
	  'make down                     stop and delete everything' \
	  'make wander-bin | local-image build only the binary, or its image' \
	  'make logs | ps                follow logs, list containers' \
	  'eval "$$(make -s env)"         point this shell at the cluster'

# -------------------------------------------------------------------------
# SOURCE BUILD
#
# The release workflow's recipe: npm and templ when missing, then the CSS and
# templ templates, then the binary. The repository's binary rule has no
# prerequisites, so it is always forced; Go's build cache keeps an unchanged
# rebuild to seconds.
# -------------------------------------------------------------------------

$(TEMPL):
	$(MAKE) -C $(REPO_ROOT) templ-deps

$(UI_DEPS): $(REPO_ROOT)/ui/package-lock.json
	cd $(REPO_ROOT)/ui && npm ci

wander-bin: $(TEMPL) $(UI_DEPS)
	$(MAKE) -s -C $(REPO_ROOT)/ui build-css
	cd $(REPO_ROOT)/ui && $(TEMPL) generate
	$(MAKE) -C $(REPO_ROOT) -B pkg/linux_amd64/wander GO_TAGS=release CGO_ENABLED=1

# --- the node image takes only /bin/wander from WANDER_IMAGE ---
local-image: wander-bin
	printf 'FROM scratch\nCOPY wander /bin/wander\n' | docker build -q -t wander-local -f - $(dir $(WANDER_BIN))
	@echo "wander-local: $$($(CLEAN_ENV) $(WANDER_BIN) version | tr '\n' ' ')"

# -------------------------------------------------------------------------
# CLUSTER
# -------------------------------------------------------------------------

.cache/images.tar:
	@mkdir -p .cache
	for i in $(IMAGES); do docker image inspect $$i >/dev/null 2>&1 || docker pull -q $$i; done
	docker save -o $@ $(IMAGES)

images: .cache/images.tar

# -------------------------------------------------------------------------
# NODE IMAGE
#
# One build for all six nodes. Compose would build a copy per service, each
# stamped with its service name, so every `up` saw a changed image and
# recreated the nodes. An unchanged rebuild keeps the same image ID here.
# -------------------------------------------------------------------------

node-image: $(UP_DEPS)
	docker build -q -t $(NODE_IMAGE) \
	  --build-arg WANDER_IMAGE=$(WANDER_IMAGE) \
	  --build-arg CONSUL_IMAGE=hashicorp/consul:$(CONSUL_VERSION) \
	  node

up: images node-image
	$(COMPOSE) up -d --wait --remove-orphans
	@echo "$(WANDER_IMAGE)" > .cache/wander-image
	@echo; echo "cluster up: wander=$(WANDER_IMAGE) consul=$(CONSUL_VERSION) vault=$(VAULT_VERSION)"
	@echo "  wander: http://10.77.0.31:4646/ui/  consul: http://10.77.0.11:8500/ui/  vault: http://10.77.0.20:8200 (token root)"

down:
	$(COMPOSE) down -v --remove-orphans
	rm -f .cache/wander-image

test: up
	$(CLEAN_ENV) ./tests.sh $(TESTS)

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

# --- prints unset lines first, then the test cluster's addresses ---
env:
	@env | grep -oE '^(NOMAD|WANDER|CONSUL|VAULT)_[A-Za-z0-9_]*' | sed 's/^/unset /'
	@echo 'export NOMAD_ADDR=http://10.77.0.31:4646 CONSUL_HTTP_ADDR=http://10.77.0.11:8500 VAULT_ADDR=http://10.77.0.20:8200 VAULT_TOKEN=root'
