// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package handlers

const shortIdentifierLength = 8

func shortIdentifier(id string) string {
	if len(id) <= shortIdentifierLength {
		return id
	}
	return id[:shortIdentifierLength]
}
