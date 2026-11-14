# Wander website

This directory contains the clean, independently designed Wander website. It
does not reuse the inherited `website/` layouts or assets.

The site is static output rendered with Go/templ and styled with Wander's
Tailwind theme. It adds no package manifest: the build reuses the repository's
pinned templ generator, standalone Tailwind binary, and existing UI plugin
installation.

```shell
make -C site build
make -C site check
make -C site serve
```

`serve` listens on `127.0.0.1:8080`. Generated files are written to `dist/`
and are intentionally ignored. The build currently prepares the canonical
`/wander/docs/` and `/wander/tutorials/` routes, but deployment remains a
separate operation until the `openwander.org` hosting target is confirmed.

## Contributing

Wander does not accept code copied from the HashiCorp Nomad repository. Read
[CONTRIBUTING.md](../CONTRIBUTING.md) before opening a pull request.
