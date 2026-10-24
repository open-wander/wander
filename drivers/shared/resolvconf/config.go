// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package resolvconf

import (
	"bufio"
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"strings"

	mobyresolvconf "github.com/moby/moby/v2/daemon/libnetwork/resolvconf"
)

// Config contains the parts of resolv.conf used by task DNS configuration.
type Config struct {
	Nameservers []string
	Searches    []string
	Options     []string
}

// SystemPath returns the host resolver configuration selected by Moby.
func SystemPath() string {
	return mobyresolvconf.Path()
}

// Read parses a resolver configuration file.
func Read(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(content), nil
}

// Parse extracts nameserver, search, and options directives. For search and
// options, the last directive wins, matching resolv.conf semantics.
func Parse(content []byte) Config {
	var config Config
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if comment := strings.IndexAny(line, "#;"); comment >= 0 {
			line = line[:comment]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "nameserver":
			if _, err := netip.ParseAddr(fields[1]); err == nil {
				config.Nameservers = append(config.Nameservers, fields[1])
			}
		case "search":
			config.Searches = append(config.Searches[:0], fields[1:]...)
		case "domain":
			config.Searches = append(config.Searches[:0], fields[1])
		case "options":
			config.Options = append(config.Options[:0], fields[1:]...)
		}
	}
	return config
}

// Write creates a resolver configuration suitable for bind-mounting into a task.
func Write(path string, nameservers, searches, options []string) error {
	var content strings.Builder
	for _, nameserver := range nameservers {
		if _, err := netip.ParseAddr(nameserver); err != nil {
			return fmt.Errorf("bad nameserver address %q: %w", nameserver, err)
		}
		fmt.Fprintf(&content, "nameserver %s\n", nameserver)
	}
	writeResolverDirective(&content, "search", searches)
	writeResolverDirective(&content, "options", options)
	return os.WriteFile(path, []byte(content.String()), 0o644)
}

func writeResolverDirective(content *strings.Builder, name string, values []string) {
	if len(values) > 0 {
		fmt.Fprintf(content, "%s %s\n", name, strings.Join(values, " "))
	}
}
