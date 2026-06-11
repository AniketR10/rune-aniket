# Unstable Build LLC ("COMPANY") CONFIDENTIAL
# Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
#
# Build targets for the llama.cpp submodule. The resulting static libraries
# are consumed by the Go package via its cgo directives.

UNAME := $(shell uname -s)
UNAME_ARCH := $(shell uname -m)

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

# LLAMACPP_OSX_ARCH lets callers cross-build the static libraries for a
# specific macOS architecture (x86_64 or arm64) from a host of the other
# architecture. When unset the build follows the host arch (no behavior
# change). Only meaningful on Darwin.
ifeq ($(UNAME),Darwin)
ifneq ($(LLAMACPP_OSX_ARCH),)
  CMAKE_FLAGS += -DCMAKE_OSX_ARCHITECTURES=$(LLAMACPP_OSX_ARCH)
  # When cross-building for a different architecture than the host, ggml's
  # default GGML_NATIVE=ON would emit -march=native / -mcpu=<host-cpu>
  # (e.g. apple-m4), which the cross compiler rejects for the target arch.
  # Disable native tuning so ggml falls back to a portable baseline for the
  # requested arch.
  ifneq ($(LLAMACPP_OSX_ARCH),$(UNAME_ARCH))
    CMAKE_FLAGS += -DGGML_NATIVE=OFF
  endif
endif
endif

# LLAMACPP_OSX_DEPLOYMENT_TARGET pins the minimum macOS the static libraries
# (and the embedded Metal shader library) are built for. GGML_METAL_MACOSX_VERSION_MIN
# is set to the same value so the .metallib compiled into libggml-metal.a targets
# the same floor as the surrounding C/C++ objects. Only meaningful on Darwin.
ifeq ($(UNAME),Darwin)
ifneq ($(LLAMACPP_OSX_DEPLOYMENT_TARGET),)
  CMAKE_FLAGS += -DCMAKE_OSX_DEPLOYMENT_TARGET=$(LLAMACPP_OSX_DEPLOYMENT_TARGET)
  CMAKE_FLAGS += -DGGML_METAL_MACOSX_VERSION_MIN=$(LLAMACPP_OSX_DEPLOYMENT_TARGET)
endif
endif

ifeq ($(LLAMACPP_CUDA),1)
  CMAKE_FLAGS += -DGGML_CUDA=ON
  # NCCL is multi-GPU collective comms — useful for distributed inference,
  # not for a desktop/edge agent. cmake auto-detects libnccl when it ships
  # in the CUDA devel image (e.g. nvidia/cuda:*-devel-ubuntu22.04), which
  # then introduces undefined references to ncclCommDestroy / ncclAllReduce
  # at link time unless we also link -lnccl. Disable explicitly.
  CMAKE_FLAGS += -DGGML_CUDA_NCCL=OFF
endif

# LLAMACPP_BLAS=1 enables the GGML BLAS backend on non-Darwin builds (Darwin
# already opts in unconditionally above via Accelerate). On linux this links
# OpenBLAS into libggml-blas.a, which the cgo bindings pull in when built
# with `-tags=llamacpp_blas`.
ifeq ($(LLAMACPP_BLAS),1)
  CMAKE_FLAGS += -DGGML_BLAS=ON
endif

# EXTRA_CMAKE_FLAGS lets callers append cmake variables without editing this
# Makefile. It is primarily used by the Linux cross-compile flow in
# deploy/rune-agent/Dockerfile to inject a CMAKE_TOOLCHAIN_FILE and override
# CMAKE_C/CXX_COMPILER for the target architecture.
CMAKE_FLAGS += $(EXTRA_CMAKE_FLAGS)

LIBS := libllama.a libggml.a libggml-base.a libggml-cpu.a \
        libllama-common.a libllama-common-base.a libcpp-httplib.a \
        libmtmd.a libserver-context.a
ifeq ($(UNAME),Darwin)
  LIBS += libggml-metal.a libggml-blas.a
endif
ifeq ($(LLAMACPP_BLAS),1)
ifneq ($(UNAME),Darwin)
  LIBS += libggml-blas.a
endif
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
		cd ../../.. && git submodule update --init --recursive llm/llamacpp/llama.cpp; \
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
ifeq ($(LLAMACPP_BLAS),1)
ifneq ($(UNAME),Darwin)
BACKEND_TARGETS += ggml-blas
endif
endif
ifeq ($(LLAMACPP_CUDA),1)
BACKEND_TARGETS += ggml-cuda
endif

clean:
	@rm -rf $(BUILD_DIR) $(OUT_DIR)
