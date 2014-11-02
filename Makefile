.SUFFIXES:

# Determine whether we're being verbose or not
export V = false
export CMD_PREFIX   = @
export NULL_REDIR = 2>/dev/null >/dev/null
ifeq ($(V),true)
	CMD_PREFIX =
	NULL_REDIR =
endif

# Default build type is debug.
TYPE ?= debug

# We ignore the ".map" file in release mode, to keep the size of our binary
# to a minimum.  In debug mode, we also always load the assets from disk.
ifeq ($(TYPE),release)
export ASSET_FLAGS   :=
else
export ASSET_FLAGS   := --debug
endif

RED     := \e[0;31m
GREEN   := \e[0;32m
YELLOW  := \e[0;33m
NOCOLOR := \e[0m

EMBED_FILES :=  $(shell find frontend/ -name '*.tmpl') \
				$(shell find frontend/css -name '*.css') \
				$(shell find frontend/js -name '*.js') \
				$(shell find frontend/fonts -type f)
BUILD_FILES  := $(patsubst frontend/%,build/%,$(EMBED_FILES))

# The final list of resources to embed in our binary.
RESOURCES    := $(BUILD_FILES)

######################################################################

.PHONY: all
all: dependencies build/imagehost


build/imagehost: descriptors.go resources.go *.go
	@printf "  $(GREEN)GO$(NOCOLOR)       $@\n"
	$(CMD_PREFIX)godep go build -o $@ .

descriptors.go: $(RESOURCES)
	@printf "  $(GREEN)ASSETS$(NOCOLOR)   $@\n"
	$(CMD_PREFIX)python ./gen_descriptors.py \
		$(ASSET_FLAGS) \
		--prefix "build/" \
		$(RESOURCES) > $@

resources.go: $(RESOURCES) build/go-bindata
	@printf "  $(GREEN)BINDATA$(NOCOLOR)  $@\n"
	$(CMD_PREFIX)build/go-bindata \
		-ignore='^.*(\.gitignore|imagehost)$$' \
		-prefix "./build" \
		-o $@ \
		$(sort $(dir $(RESOURCES)))

build/go-bindata:
	@printf "  $(GREEN)DEPS(B)$(NOCOLOR)  $@\n"
	$(CMD_PREFIX)godep go build -o $@ github.com/jteeuwen/go-bindata/go-bindata

######################################################################

build/%.tmpl: frontend/%.tmpl
	@printf "  $(GREEN)CP$(NOCOLOR)       $< ==> $@\n"
	$(CMD_PREFIX)cp $< $@

build/css/%: frontend/css/%
	@printf "  $(GREEN)CP$(NOCOLOR)       $< ==> $@\n"
	@mkdir -p $(dir $@)
	$(CMD_PREFIX)cp $< $@

build/fonts/%: frontend/fonts/%
	@printf "  $(GREEN)CP$(NOCOLOR)       $< ==> $@\n"
	@mkdir -p $(dir $@)
	$(CMD_PREFIX)cp $< $@

######################################################################

# This is a phony target that checks to ensure our various dependencies are installed
.PHONY: dependencies
dependencies:
	@command -v godep      >/dev/null 2>&1 || { printf >&2 "godep is not installed, exiting...\n"; exit 1; }

######################################################################

.PHONY: clean
CLEAN_FILES := build/imagehost build/go-bindata resources.go descriptors.go $(RESOURCES)
clean:
	@printf "  $(YELLOW)RM$(NOCOLOR)       $(CLEAN_FILES)\n"
	$(CMD_PREFIX)$(RM) -r $(CLEAN_FILES)

.PHONY: env
env:
	@echo "RESOURCES = $(RESOURCES)"
