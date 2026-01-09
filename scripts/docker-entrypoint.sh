#!/usr/bin/env ash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


case "$1" in
  "agent" )
    if [[ -z "${WANDER_SKIP_DOCKER_IMAGE_WARN}" ]]
    then
      echo "===================================================================================="
      echo "!! Running Wander clients inside Docker containers is not supported.              !!"
      echo "!! Refer to https://openwander.org/wander/docs/install for more information.      !!"
      echo "!! Set the WANDER_SKIP_DOCKER_IMAGE_WARN environment variable to skip this warn.  !!"
      echo "===================================================================================="
      echo ""
      sleep 2
    fi
esac

exec wander "$@"
