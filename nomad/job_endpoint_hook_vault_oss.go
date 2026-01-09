// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package nomad

import (
	"fmt"
	"strings"

	vapi "github.com/hashicorp/vault/api"
	"github.com/open-wander/wander/nomad/structs"
)

// validateNamespaces returns an error if the job contains multiple Vault
// namespaces.
func (jobVaultHook) validateNamespaces(
	blocks map[string]map[string]*structs.Vault,
	token *vapi.Secret,
) error {

	requestedNamespaces := structs.VaultNamespaceSet(blocks)
	if len(requestedNamespaces) > 0 {
		return fmt.Errorf("%w, Namespaces: %s", ErrMultipleNamespaces, strings.Join(requestedNamespaces, ", "))
	}
	return nil
}
