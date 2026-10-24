#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# Install Go templ for template generation
go install github.com/a-h/templ/cmd/templ@latest

# Install air for hot-reload development
go install github.com/air-verse/air@latest

# Setup UI dependencies (Tailwind CLI + npm packages for plugins)
cd /home/vagrant/wander/ui-htmx && make setup
