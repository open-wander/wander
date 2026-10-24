#!/usr/bin/env ash
# shellcheck shell=dash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


case "$1" in
  "agent" )
    if [ -z "${WANDER_SKIP_DOCKER_IMAGE_WARN}" ] && [ -z "${NOMAD_SKIP_DOCKER_IMAGE_WARN}" ]
    then
      echo "===================================================================================="
      echo "!! Running Wander clients inside Docker containers is not supported.              !!"
      echo "!! Refer to https://openwander.org/wander/docs/install for more information.      !!"
      echo "!! Set WANDER_SKIP_DOCKER_IMAGE_WARN (or the NOMAD_* compatibility alias) to skip. !!"
      echo "===================================================================================="
      echo ""
      sleep 2
    fi
esac

exec wander "$@"
