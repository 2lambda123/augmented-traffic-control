# You must have a working go environment in order to build atc.
# See https://golang.org/doc/code.html


# Path to install files under
PREFIX = /usr/local

GO = $(shell which go)
BUILD = $(GO) build
# for static compilation:
ifdef STATICBUILD
	BUILD += --ldflags '-extldflags "-static"'
endif

TEST = $(GO) test -v
VET = $(GO) vet
FMT = $(GO) fmt
GET = $(GO) get
LIST = $(GO) list
BINGEN = $(GOPATH)/bin/go-bindata # github.com/jteeuwen/go-bindata
THRIFT = thrift
THRIFT_GO_FLAGS=thrift_import=thrift/lib/go/thrift
NPM = npm

# The $(GO) project root
PROJECT = github.com/facebook/augmented-traffic-control
SRC = ${PROJECT}/src
CMD = ${PROJECT}/cmd


USERID = $(shell id -u)

.PHONY: all bin tests ui lint
bin: bin/atcd bin/atc_api bin/atc
all: vendor/atc_thrift lint ui tests bin
tests: test-daemon test-api test-shaping
ui: src/assets/bindata.go
lint: lint-ui lint-daemon lint-api lint-client

###
### Binaries
###

bin/atcd: cmd/atcd/*.go cmd/atcd/cli/*.go vendor/atc_thrift src/daemon/*.go src/log/*.go src/shaping/*.go
	@mkdir -p bin
	$(BUILD) -o $@ ${CMD}/atcd

bin/atc_api: cmd/atc_api/*.go cmd/atc_api/cli/*.go vendor/atc_thrift src/api/*.go src/log/*.go src/assets/bindata.go
	@mkdir -p bin
	$(BUILD) -o $@ ${CMD}/atc_api

bin/atc: cmd/atc/*.go cmd/atc/cli/*.go vendor/atc_thrift src/log/*.go
	@mkdir -p bin
	$(BUILD) -o $@ ${CMD}/atc

vendor/atc_thrift: if/atc_thrift.thrift
	mkdir -p vendor
	$(THRIFT) --out vendor/ --gen go:$(THRIFT_GO_FLAGS) if/atc_thrift.thrift

###
### UI
###

.PHONY: npm_env

static/js/index.js: src/react/jsx/*.js
	cd src/react && $(NPM) run build-js

src/assets/bindata.go: static/js/index.js
	mkdir -p src/assets/
	$(BINGEN) -pkg assets -o $@ static/...

npm_env:
	cd src/react && $(NPM) install

###
### Tests
###

.PHONY: test-daemon test-shaping test-api

test-daemon:
	$(TEST) ${SRC}/daemon
	$(TEST) ${SRC}/atcd

test-shaping:
	@echo "[31mRunning shaping tests as root.[39m"
ifeq ($(USERID),0)
	$(TEST) ${SRC}/shaping
else
	sudo PATH=${PATH} GOROOT=${GOROOT} GOPATH=${GOPATH} $(TEST) ${SRC}/shaping
endif

test-api:
	$(TEST) ${SRC}/api
	$(TEST) ${SRC}/atc_api

docker-run:
	docker build -t atc .
	docker run -ti \
		--rm \
		--privileged \
		-v "$(PWD)":/usr/src/myapp \
		-w /usr/src/myapp \
		-p 9090:9090 \
		-p 8080:8080 \
		atc bash

docker-automated-test:
	docker build -t atc .
	docker run -ti \
		--rm \
		--privileged \
		-v "$(PWD)":/usr/src/myapp \
		-w /usr/src/myapp \
		-p 9090:9090 \
		-p 8080:8080 \
		-e RESET_DB=true \
		-e RESET_LOGS=true \
		atc bash -c "utils/automated-test.sh"

###
### Lint
###

.PHONY: lint-ui lint-daemon lint-api lint-client

lint-ui:
	cd src/react && $(NPM) run lint

lint-daemon:
	@$(FMT) ${SRC}/shaping ${SRC}/daemon ${SRC}/atcd
	$(VET) ${SRC}/shaping
	$(VET) ${SRC}/daemon
	$(VET) ${SRC}/atcd

lint-api:
	@$(FMT) ${SRC}/api ${SRC}/atc_api
	$(VET) ${SRC}/api
	$(VET) ${SRC}/atc_api

lint-client:
	@$(FMT) ${SRC}/atc
	$(VET) ${SRC}/atc

###
### Helpers
###

.PHONY: install clean

# Removed compiled binaries
clean:
	rm -rf bin/

# Remove all generated files and binaries
clean-all: clean
	rm -rf vendor/atc_thrift src/assets/bindata.go

# Copy built binaries into /usr/local/bin/
install:
	cp bin/atcd bin/atc_api "$(PREFIX)/bin/"
