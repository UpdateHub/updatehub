# updatehub
# Copyright (C) 2017 O.S. Systems Sofware LTDA
# Copyright (C) 2017 Cloudflare
#
# SPDX-License-Identifier:     GPL-2.0

# The import path is where your repository can be found.
# To import subpackages, always prepend the full import path.
# If you change this, run `make clean`. Read more: https://git.io/vM7zV
IMPORT_PATH := github.com/updatehub/updatehub

# V := 1 # When V is set, print commands and build progress.

# Space separated patterns of packages to skip in list, test, format.
IGNORED_PACKAGES := /vendor/

.PHONY: all
all: build

.PHONY: build
build: .GOPATH/.ok updatehub updatehub-server updatehub-ctl

.PHONY: updatehub
updatehub: .GOPATH/.ok vendor
	@echo -n "building updatehub… "
	$Q go install $(if $V,-v -x) $(VERSION_FLAGS) $(IMPORT_PATH)/cmd/updatehub
	@echo "done"

.PHONY: updatehub-server vendor
updatehub-server: .GOPATH/.ok
	@echo -n "building updatehub-server… "
	$Q go install $(if $V,-v -x) $(VERSION_FLAGS) $(IMPORT_PATH)/cmd/updatehub-server
	@echo "done"

.PHONY: updatehub-ctl vendor
updatehub-ctl: .GOPATH/.ok
	@echo -n "building updatehub-ctl… "
	$Q go install $(if $V,-v -x) $(VERSION_FLAGS) $(IMPORT_PATH)/cmd/updatehub-ctl
	@echo "done"

##### =====> Utility targets <===== #####

.PHONY: clean test list coverage format

clean:
	$Q rm -rf bin .GOPATH

MACHINE_ARCH := $(shell uname --machine)
ifneq ($(MACHINE_ARCH),i686)
	TEST_RACE := -race
endif

test: .GOPATH/.ok
	$Q go test $(if $V,-v) -i $(TEST_RACE) $(allpackages) # install -race libs to speed up next run
ifndef CI
	$Q go vet $(allpackages)
	$Q GODEBUG=cgocheck=2 go test $(TEST_RACE) $(allpackages)
else
	$Q ( go vet $(allpackages); echo $$? ) | \
	    tee .GOPATH/test/vet.txt | sed '$$ d'; exit $$(tail -1 .GOPATH/test/vet.txt)
	$Q ( GODEBUG=cgocheck=2 go test -v $(TEST_RACE) $(allpackages); echo $$? ) | \
	    tee .GOPATH/test/output.txt | sed '$$ d'; exit $$(tail -1 .GOPATH/test/output.txt)
endif

list: .GOPATH/.ok
	@echo $(allpackages) | tr " " "\n"

coverage: .GOPATH/.ok bin/gocovmerge vendor
	@echo "NOTE: make coverage does not exit 1 on failure, don't use it to check for tests success!"
	$Q rm -f .GOPATH/coverage/*.out .GOPATH/coverage/all.merged
	$(if $V,@echo "-- go test -coverpkg=./... -coverprofile=.GOPATH/coverage/... ./...")
	@for MOD in $(allpackages); do \
		go test -coverpkg=`echo $(allpackages)|tr " " ","` \
			-coverprofile=.GOPATH/coverage/unit-`echo $$MOD|tr "/" "_"`.out \
			$$MOD 2>&1 | grep -v "no packages being tested depend on"; \
	done
	$Q ./bin/gocovmerge .GOPATH/coverage/*.out > .GOPATH/coverage/all.merged
ifndef CI
	$Q go tool cover -html .GOPATH/coverage/all.merged
else
	$Q go tool cover -html .GOPATH/coverage/all.merged -o .GOPATH/coverage/all.html
endif
	@echo ""
	@echo "=====> Total test coverage: <====="
	@echo ""
	$Q go tool cover -func .GOPATH/coverage/all.merged

format: .GOPATH/.ok bin/goimports
	$Q find .GOPATH/src/$(IMPORT_PATH)/ -iname \*.go | grep -v \
	    -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)) | xargs ./bin/goimports -w

##### =====> Internals <===== #####

VERSION          := $(shell git describe --tags --always --dirty="-dirty")
VERSION_FLAGS    := -ldflags='-X "main.gitversion=$(VERSION)"'

# cd into the GOPATH to workaround ./... not following symlinks
_allpackages = $(shell ( cd $(CURDIR)/.GOPATH/src/$(IMPORT_PATH) && \
    GOPATH=$(CURDIR)/.GOPATH go list ./... 2>&1 1>&3 | \
    grep -v -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)) 1>&2 ) 3>&1 | \
    grep -v -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)))

# memoize allpackages, so that it's executed only once and only if used
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)

export GOPATH := $(CURDIR)/.GOPATH
unexport GOBIN

Q := $(if $V,,@)

.GOPATH/.ok:
	$Q mkdir -p "$(dir .GOPATH/src/$(IMPORT_PATH))"
	$Q ln -s ../../../.. ".GOPATH/src/$(IMPORT_PATH)"
	$Q mkdir -p .GOPATH/test .GOPATH/coverage
	$Q mkdir -p bin
	$Q ln -s ../bin .GOPATH/bin
	$Q touch $@

bin/gocovmerge: .GOPATH/.ok
	$Q go get github.com/wadey/gocovmerge
bin/goimports: .GOPATH/.ok
	$Q go get golang.org/x/tools/cmd/goimports
bin/glide: .GOPATH/.ok
	$Q go get github.com/Masterminds/glide
bin/gometalinter: .GOPATH/.ok
	$Q go get github.com/alecthomas/gometalinter
	$Q ./bin/gometalinter --install

.PHONY: vendor lint

vendor: .GOPATH/.ok bin/glide
	@test -d ./vendor/ || \
		{ ./bin/glide install; }

lint: .GOPATH/.ok bin/gometalinter
	@for MOD in $(allpackages); do \
		echo ""; \
		echo "=====> linting $$MOD: <====="; \
		echo ""; \
		./bin/gometalinter --aggregate --deadline=30s `echo $$MOD | sed "s,$(IMPORT_PATH)/,,g"`; \
	done
