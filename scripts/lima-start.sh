#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# Lima VM convenience script for Wander development
# This is an alternative to Vagrant for Apple Silicon Macs.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VM_NAME="wander"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

check_lima() {
    if ! command -v limactl &> /dev/null; then
        error "Lima is not installed. Install it with: brew install lima"
    fi
}

create_vm() {
    info "Creating Lima VM '${VM_NAME}'..."
    cd "$PROJECT_DIR"
    limactl create --name="${VM_NAME}" lima.yaml
}

start_vm() {
    info "Starting Lima VM '${VM_NAME}'..."
    limactl start "${VM_NAME}"
}

stop_vm() {
    info "Stopping Lima VM '${VM_NAME}'..."
    limactl stop "${VM_NAME}"
}

delete_vm() {
    info "Deleting Lima VM '${VM_NAME}'..."
    limactl delete "${VM_NAME}" --force
}

shell_vm() {
    exec limactl shell "${VM_NAME}"
}

status_vm() {
    limactl list
}

usage() {
    cat <<EOF
Usage: $0 <command>

Commands:
  up        Create and start the VM (first time setup)
  start     Start an existing VM
  stop      Stop the VM
  ssh       SSH into the VM
  status    Show VM status
  destroy   Delete the VM completely
  help      Show this help message

Examples:
  $0 up       # First time: create and provision the VM
  $0 ssh      # Connect to the VM
  $0 stop     # Stop the VM when done
  $0 destroy  # Remove the VM completely

EOF
}

# Main
check_lima

case "${1:-}" in
    up)
        if limactl list --json | grep -q "\"name\":\"${VM_NAME}\""; then
            warn "VM '${VM_NAME}' already exists. Starting it..."
            start_vm
        else
            create_vm
            start_vm
        fi
        info "VM is ready! Run '$0 ssh' to connect."
        ;;
    start)
        start_vm
        ;;
    stop)
        stop_vm
        ;;
    ssh|shell)
        shell_vm
        ;;
    status)
        status_vm
        ;;
    destroy|delete)
        stop_vm 2>/dev/null || true
        delete_vm
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        usage
        exit 1
        ;;
esac
