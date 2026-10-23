# Embedded browser asset provenance

All hashes are SHA-256 over the currently tracked file.

| Tracked asset | Upstream/version | License | SHA-256 | Verification |
| --- | --- | --- | --- | --- |
| `ui/static/js/alpine.min.js` | `@alpinejs/csp@3.15.4` | MIT | `e96c88f4f2b2aa90e3bccfec43c020cb877d81edad72ffcbdb0a9016a2bf67b0` | Byte-identical to npm `dist/cdn.min.js`; npm integrity `sha512-S8LyVp3BR+Yp9hbUl6HlOcopEDFBqtOdtOtJT6i++ziJk0WPMG482bGZvTI7+4hEwpC7N9y1+mXoNb7ryfGm2A==`, git commit `7ce2be8755e67ffd1d7481723e7805e3b87cd170`. |
| `ui/static/js/htmx.min.js` | `htmx.org@2.0.8` | 0BSD | `22283ef68cb7545914f0a88a1bdedc7256a703d1d580c1d255217d0a50d31313` | Byte-identical to npm `dist/htmx.min.js`; npm integrity `sha512-fm297iru0iWsNJlBrjvtN7V9zjaxd+69Oqjh4F/Vq9Wwi2kFisLcrLCiv5oBX0KLfOX/zG8AUo9ROMU5XUB44Q==`. |
| `ui/static/js/xterm.min.js` | `@xterm/xterm@5.5.0` | MIT | `4196e242ef1cf4c2adead8d97f4a772a69576076f70b095e004b4abbb049e7bf` | After removing the six-line jsDelivr banner, byte-identical to npm `lib/xterm.js`; npm integrity `sha512-hqJHYaQb5OptNunnyAnkHyM8aCjZ1MEIDTQu1iIbbTD/xops91NB5yq1ZK/dC2JDbVWtF23zUtl9JE2NqwT87A==`. |
| `ui/static/css/xterm.min.css` | `@xterm/xterm@5.5.0` | MIT | `f7f724aea2bb620a6482bfb8e4bdecfae1152b0c7facef55fbda61f3b6cfedb2` | jsDelivr clean-css 5.3.3 minification of npm `css/xterm.css`, as declared in its retained banner. |
| `ui/static/js/xterm-addon-fit.min.js` | `@xterm/addon-fit@0.10.0` | MIT | `a6a7bbb33569f16aa3e18d71425e34d035fc89a0b7e8cba084f8855f91aa38f1` | After removing the six-line jsDelivr banner, byte-identical to npm `lib/addon-fit.js`; npm integrity `sha512-UFYkDm4HUahf2lnEyHvio51TNGiLK66mqP2JoATy7hRZeXaGMRDr00JiSF7m63vR5WKATF605yEggJKsw0JpMQ==`. |

`ui/static/js/codemirror-bundle.js` is not a vendored CodeMirror distribution.
Despite the historical filename and heading, it is a Wander-authored lightweight
syntax highlighter/editor shim introduced in the replacement UI. It should be
renamed later to avoid implying third-party CodeMirror provenance.

The exact browser license texts retained under `third-party/` came from the
corresponding npm tarballs, except Alpine's (the scoped package omits the text),
which came from the package's exact git commit recorded above.
