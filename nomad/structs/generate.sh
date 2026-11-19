#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -e

# The generated code follows the order of these files, so sort them the same
# way in every locale.
export LC_ALL=C

FILES="$(ls ./*.go | grep -v -e _test.go -e .generated.go | tr '\n' ' ')"
../../scripts/codecgen.sh \
    -c github.com/hashicorp/go-msgpack/codec \
    -st codec \
    -d 100 \
    -t codegen_generated \
    -o structs.generated.go \
    -nr="(^ACLCache$)|(^IdentityClaims$)" \
    ${FILES}
