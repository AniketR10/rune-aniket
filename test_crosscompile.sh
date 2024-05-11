#!/usr/bin/env sh

# Test script checking that all expected os/arch compile properly.
# Does not actually test the logic, just the compilation so we make sure we don't break code depending on the lib.

echo2() {
  echo $@ >&2
}

trap end 0
end() {
  [ "$?" = 0 ] && echo2 "Pass." || (echo2 "Fail."; exit 1)
}

cross() {
  os=$1
  shift
  echo2 "Build for $os."
  for arch in $@; do
    echo2 "  - $os/$arch"
    GOOS=$os GOARCH=$arch go build -o bin/$os-$arch-six ./cmd/six
  done
  echo2
}

set -e

cross linux     amd64 386 arm arm64 ppc64 ppc64le s390x mips mipsle mips64 mips64le riscv64
cross darwin    amd64 arm64
cross freebsd   amd64 386 arm arm64 riscv64
cross netbsd    amd64 386 arm arm64
cross openbsd   amd64 386 arm arm64
cross dragonfly amd64
cross solaris   amd64

# TODO
# cross windows amd64 386 arm
