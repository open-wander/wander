// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/open-wander/wander/api"
)

func Parse(path string, r io.Reader) (*api.Job, error) {
	if path == "" {
		if f, ok := r.(*os.File); ok {
			path = f.Name()
		}
	}

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}

	return ParseWithConfig(&ParseConfig{
		Path:    path,
		Body:    buf.Bytes(),
		AllowFS: false,
		Strict:  true,
	})
}

func ParseWithConfig(args *ParseConfig) (*api.Job, error) {
	args.normalize()

	c := newJobConfig(args)
	err := decode(c)
	if err != nil {
		return nil, err
	}

	normalizeJob(c)
	return c.Job, nil
}

type ParseConfig struct {
	Path    string
	BaseDir string

	// Body is the HCL body
	Body []byte

	// AllowFS enables HCL functions that require file system access
	AllowFS bool

	// ArgVars is the CLI -var arguments
	ArgVars []string

	// VarFiles is the paths of variable data files that should be read during
	// parsing.
	VarFiles []string

	// VarContent is the content of variable data known without reading an
	// actual var file during parsing.
	VarContent string

	// Envs represent process environment variable
	Envs []string

	Strict bool

	// parsedVarFiles represent parsed HCL AST of the passed EnvVars
	parsedVarFiles []*hcl.File
}

func (c *ParseConfig) normalize() {
	if c.BaseDir == "" {
		c.BaseDir = filepath.Dir(c.Path)
	}
}

func decode(c *jobConfig) error {
	config := c.ParseConfig

	file, diags := parseHCLOrJSON(config.Body, config.Path)

	for _, varFile := range config.VarFiles {
		parsedVarFile, ds := parseFile(varFile)
		if parsedVarFile == nil || ds.HasErrors() {
			return fmt.Errorf("unable to parse var file: %v", ds.Error())
		}

		config.parsedVarFiles = append(config.parsedVarFiles, parsedVarFile)
		diags = append(diags, ds...)
	}

	if config.VarContent != "" {
		hclFile, hclDiagnostics := parseHCLOrJSON([]byte(config.VarContent), "input.hcl")
		if hclDiagnostics.HasErrors() {
			return fmt.Errorf("unable to parse var content: %v", hclDiagnostics.Error())
		}
		config.parsedVarFiles = append(config.parsedVarFiles, hclFile)
	}

	// Return early if the input job or variable files are not valid.
	// Decoding and evaluating invalid files may result in unexpected results.
	if diags.HasErrors() {
		return diags
	}

	diags = append(diags, c.decodeBody(file.Body)...)

	if diags.HasErrors() {
		var str strings.Builder
		for i, diag := range diags {
			if i != 0 {
				str.WriteByte('\n')
			}
			str.WriteString(diag.Error())
		}
		return errors.New(str.String())
	}

	diags = append(diags, decodeMapInterfaceType(&c.Job, c.EvalContext())...)
	diags = append(diags, decodeMapInterfaceType(&c.Tasks, c.EvalContext())...)
	diags = append(diags, decodeMapInterfaceType(&c.Vault, c.EvalContext())...)

	if diags.HasErrors() {
		return diags
	}

	return nil
}

func parseFile(path string) (*hcl.File, hcl.Diagnostics) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Failed to read file",
				Detail:   fmt.Sprintf("failed to read %q: %v", path, err),
			},
		}
	}

	return parseHCLOrJSON(body, path)
}

func parseHCLOrJSON(src []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	if isJSON(src) {
		return hcljson.Parse(src, filename)
	}

	return hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
}

func isJSON(src []byte) bool {
	for _, c := range src {
		if c == ' ' {
			continue
		}

		return c == '{'
	}
	return false
}
