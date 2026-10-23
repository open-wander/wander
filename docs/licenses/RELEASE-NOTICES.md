# Wander release notices

Wander is distributed under the Mozilla Public License 2.0. The complete
project license is included as `LICENSE.txt` beside this file in release
archives and installed packages.

The `licenses/go/` directory is generated from the exact Go dependency closure
for the release target. It contains the license and notice material found by
`github.com/google/go-licenses@v1.6.0`. When a dependency's license requires
corresponding source to accompany redistribution, the exporter also places that
source in the dependency's directory.

The effective Kubernetes authentication module for Vault is a local module
replacement and is therefore added explicitly under
`licenses/go/github.com/hashicorp/vault/api/auth/kubernetes/`.

License texts retained for browser assets embedded in the Wander binary are
included under `licenses/browser/`. Their versions, upstream locations, and
integrity hashes are documented in the Wander source tree at
`docs/licenses/browser-assets.md`.

These notices describe third-party material; they do not change the license of
Wander itself. The corresponding source for this Wander release is available
from <https://github.com/open-wander/wander> at the release revision.
