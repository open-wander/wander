// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package components

import "net/url"

// ScopedURL adds the selected namespace and region to an internal UI or API
// path without discarding either dimension when both are present.
func ScopedURL(path, namespace, region string) string {
	query := url.Values{}
	if namespace != "" {
		query.Set("namespace", namespace)
	}
	if region != "" {
		query.Set("region", region)
	}
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}
