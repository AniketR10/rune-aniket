# Unstable Build LLC ("COMPANY") CONFIDENTIAL
# Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
#
# Build targets for the llama.cpp submodule. The resulting static libraries
# are consumed by the Go package via its cgo directives.

UNAME := $(shell uname -s)

SRC_DIR := llama.cpp
BUILD_DIR := $(SRC_DIR)/_build
OUT_DIR := libs

CMAKE ?= cmake
JOBS ?= 4

CMAKE_FLAGS := \
  -DLLAMA_BUILD_TESTS=OFF \
  -DLLAMA_BUILD_EXAMPLES=OFF \
  -DLLAMA_BUILD_TOOLS=ON \
  -DLLAMA_BUILD_SERVER=ON \
  -DLLAMA_BUILD_WEBUI=OFF \
  -DBUILD_SHARED_LIBS=OFF \
  -DLLAMA_CURL=OFF \
  -DLLAMA_BUILD_COMMON=ON \
  -DLLAMA_OPENSSL=OFF \
  -DLLAMA_LLGUIDANCE=OFF \
  -DGGML_BUILD_TESTS=OFF \
  -DGGML_BUILD_EXAMPLES=OFF

ifeq ($(UNAME),Darwin)
  CMAKE_FLAGS += -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON -DGGML_BLAS=ON
endif

ifeq ($(LLAMACPP_CUDA),1)
  CMAKE_FLAGS += -DGGML_CUDA=ON
endif

LIBS := libllama.a libggml.a libggml-base.a libggml-cpu.a \
        libllama-common.a libllama-common-base.a libcpp-httplib.a \
        libmtmd.a libserver-context.a
ifeq ($(UNAME),Darwin)
  LIBS += libggml-metal.a libggml-blas.a
endif
ifeq ($(LLAMACPP_CUDA),1)
  LIBS += libggml-cuda.a
endif

OUT_LIBS := $(addprefix $(OUT_DIR)/,$(LIBS))

.PHONY: all clean libs configure build init reconfigure

all: libs

libs: $(OUT_LIBS)

$(OUT_LIBS): build
	@mkdir -p $(OUT_DIR)
	@find $(BUILD_DIR) -name '$(notdir $@)' -exec cp {} $@ \;
	@test -f $@ || (echo "missing $@"; exit 1)

# init brings in the llama.cpp submodule when the tree is empty (e.g. fresh
# clone without `git submodule update --init --recursive`).
init:
	@if [ ! -f $(SRC_DIR)/CMakeLists.txt ]; then \
		echo "llama.cpp submodule not initialised — running git submodule update --init --recursive"; \
		cd ../../../.. && git submodule update --init --recursive cmd/rune-agent/llm/llamacpp/llama.cpp; \
	fi

configure: init $(BUILD_DIR)/CMakeCache.txt

$(BUILD_DIR)/CMakeCache.txt: init
	@mkdir -p $(BUILD_DIR)
	@cd $(BUILD_DIR) && $(CMAKE) .. $(CMAKE_FLAGS)

# reconfigure wipes the cmake cache so CMAKE_FLAGS changes take effect. Use
# this when pulling a change that adjusts the flag set (e.g. enabling
# llama-common / toggling LLAMA_OPENSSL).
reconfigure:
	@rm -f $(BUILD_DIR)/CMakeCache.txt

build: configure
	@$(CMAKE) --build $(BUILD_DIR) --target \
	    llama ggml ggml-base ggml-cpu \
	    llama-common llama-common-base cpp-httplib mtmd server-context \
	    $(BACKEND_TARGETS) -j$(JOBS)

ifeq ($(UNAME),Darwin)
BACKEND_TARGETS := ggml-metal ggml-blas
endif
ifeq ($(LLAMACPP_CUDA),1)
BACKEND_TARGETS += ggml-cuda
endif

clean:
	@rm -rf $(BUILD_DIR) $(OUT_DIR)
