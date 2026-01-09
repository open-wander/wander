# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

schema = "1"

project "wander" {
  team = "wander"

  github {
    organization = "open-wander"
    repository   = "wander"

    release_branches = [
      "main",
      "release/**",
    ]
  }
}

event "build" {
  action "build" {
    organization = "open-wander"
    repository   = "wander"
    workflow     = "build"
  }
}

# Note: The HashiCorp CRT workflows (crt-workflows-common) have been removed
# as they are specific to HashiCorp's internal release infrastructure.
# Wander uses GitHub Actions for CI/CD - see .github/workflows/
