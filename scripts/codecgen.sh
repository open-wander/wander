#!/usr/bin/env bash
# Copyright (c) Open Wander contributors.
# SPDX-License-Identifier: MPL-2.0
#
# Runs codecgen with the given arguments from the calling package directory.
#
# The generator in the pinned go-msgpack release declares a base64 alphabet
# that lists '_' twice. Current Go panics on a repeated symbol, so the
# generator stops before it writes anything. No later 1.x release exists.
#
# This script runs codecgen against a temporary copy of that module in which
# the last symbol is encoded as '-' and then folded back to '_'. The generated
# names are the same as with the original alphabet. The repository's go.mod,
# the module cache and the built binaries are not changed.

set -euo pipefail

# The order of the generated code follows the order of the input files, and
# file names sort differently outside the C locale.
export LC_ALL=C

module=github.com/hashicorp/go-msgpack

if ! command -v codecgen >/dev/null; then
    echo "codecgen not found; run 'make deps' first" >&2
    exit 1
fi

root="$(go list -m -f '{{.Dir}}')"
source_dir="$(go list -m -f '{{.Dir}}' "${module}")"
if [[ -z "${source_dir}" ]]; then
    go mod download "${module}"
    source_dir="$(go list -m -f '{{.Dir}}' "${module}")"
fi

work="$(mktemp -d)"
trap 'chmod -R u+w "${work}" 2>/dev/null; rm -rf "${work}"' EXIT

cp -R "${source_dir}" "${work}/go-msgpack"
chmod -R u+w "${work}/go-msgpack"

generator="${work}/go-msgpack/codec/gen.go"
perl -pi -e 's/base64\.NewEncoding\("([A-Za-z0-9]+)__"\)/foldedAlphabet{base64.NewEncoding("${1}_-")}/' "${generator}"
if [[ "$(grep -c 'foldedAlphabet{' "${generator}")" != "1" ]]; then
    echo "codecgen.sh: the generator alphabet was not found in ${module}; has the pinned version changed?" >&2
    exit 1
fi

cat > "${work}/go-msgpack/codec/gen_folded_alphabet.go" <<'GO'
// +build codecgen.exec

package codec

import "encoding/base64"

// foldedAlphabet encodes with 64 distinct symbols and then maps '-' to '_',
// which gives the output of the original alphabet that listed '_' twice.
type foldedAlphabet struct{ *base64.Encoding }

func (f foldedAlphabet) Encode(dst, src []byte) {
	f.Encoding.Encode(dst, src)
	for i := range dst {
		if dst[i] == '-' {
			dst[i] = '_'
		}
	}
}
GO

cp "${root}/go.mod" "${root}/go.sum" "${work}/"
printf '\nreplace %s => %s\n' "${module}" "${work}/go-msgpack" >> "${work}/go.mod"

GOFLAGS="${GOFLAGS:-} -modfile=${work}/go.mod" codecgen "$@"
