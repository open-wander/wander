# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# docker.io/library/busybox:1.36.0
# When pinning use the multi-arch manifest list, `docker buildx imagetools inspect ...`
FROM docker.io/library/busybox@sha256:9e2bbca079387d7965c3a9cee6d0c53f4f4e63ff7637877a83c4c05f2a666112 AS release

ARG PRODUCT_NAME=wander
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
ARG WANDER_BINARY=dist/wander

LABEL org.opencontainers.image.title=${PRODUCT_NAME}
LABEL org.opencontainers.image.version=${PRODUCT_VERSION}
LABEL org.opencontainers.image.revision=${PRODUCT_REVISION}
LABEL org.opencontainers.image.source="https://github.com/open-wander/wander"
LABEL org.opencontainers.image.licenses="MPL-2.0"

COPY ${WANDER_BINARY} /bin/wander
COPY dist/LICENSE.txt /licenses/wander/LICENSE.txt
COPY dist/THIRD_PARTY_NOTICES.md /licenses/wander/THIRD_PARTY_NOTICES.md
COPY dist/licenses /licenses/wander/licenses
COPY ./scripts/docker-entrypoint.sh /

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]
