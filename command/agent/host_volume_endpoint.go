// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/open-wander/wander/nomad/structs"
)

// hostVolumesList serves GET /v1/volumes?type=host.
func (s *HTTPServer) hostVolumesList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.HostVolumeListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.NodeID = query.Get("node_id")
	args.NodePool = query.Get("node_pool")

	var out structs.HostVolumeListResponse
	if err := s.agent.RPC("HostVolume.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volumes == nil {
		out.Volumes = make([]*structs.DynamicHostVolumeStub, 0)
	}
	return out.Volumes, nil
}

// HostVolumeSpecificRequest serves /v1/volume/host/:volume_id.
func (s *HTTPServer) HostVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/host/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) != 1 || tokens[0] == "" {
		return nil, CodedError(404, resourceNotFoundErr)
	}

	// Volume IDs are UUIDs, so reserving these two words costs nothing.
	switch tokens[0] {
	case "create":
		return s.hostVolumeWrite("HostVolume.Create", resp, req)
	case "register":
		return s.hostVolumeWrite("HostVolume.Register", resp, req)
	}

	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return s.hostVolumeGet(tokens[0], resp, req)
}

func (s *HTTPServer) hostVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.HostVolumeGetRequest{ID: id}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.HostVolumeGetResponse
	if err := s.agent.RPC("HostVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}
	return out.Volume, nil
}

const (
	hostVolumeErrBody          = "invalid request body"
	hostVolumeErrMissingVolume = "missing volume"

	// hostVolumeMaxBody bounds the body of a create or register.
	hostVolumeMaxBody = 1 << 20

	// Bounds on the unknown field names echoed back in Warnings.
	hostVolumeMaxWarnedFields = 14
	hostVolumeMaxWarnedName   = 64

	hostVolumeWarnPolicyOverride = "PolicyOverride has no effect: Wander has no policy engine"
)

// hostVolumeWriteBody is the body of a create or register. It is decoded into
// this type, not into the RPC request, so that no key of the body can set the
// token, region, namespace or idempotency token of the request.
type hostVolumeWriteBody struct {
	Volume         *structs.DynamicHostVolume
	PolicyOverride bool
}

// hostVolumeWriteResponse is the reply of a create or register. Warnings is a
// pointer so that no warning encodes as null.
type hostVolumeWriteResponse struct {
	Volume   *structs.DynamicHostVolume
	Warnings *string
}

// hostVolumeWrite serves PUT and POST on /v1/volume/host/create and
// /v1/volume/host/register. A volume with an ID updates the stored volume.
func (s *HTTPServer) hostVolumeWrite(rpc string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPut, http.MethodPost:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	body, unknown, err := decodeHostVolumeWriteBody(req)
	if err != nil {
		return nil, CodedError(400, hostVolumeErrBody)
	}
	if body.Volume == nil {
		return nil, CodedError(400, hostVolumeErrMissingVolume)
	}

	var wr structs.WriteRequest
	s.parseWriteRequest(req, &wr)
	// With no ?namespace, the namespace of the volume is the request
	// namespace, as for a job. Two different namespaces are refused by the RPC.
	if req.URL.Query().Get("namespace") == "" && body.Volume.Namespace != "" {
		wr.Namespace = body.Volume.Namespace
	}

	var warnings []string
	if body.PolicyOverride {
		warnings = append(warnings, hostVolumeWarnPolicyOverride)
	}
	if len(unknown) > 0 {
		warnings = append(warnings, "ignored unknown fields: "+formatHostVolumeFields(unknown))
	}
	reply := hostVolumeWriteResponse{}
	if len(warnings) > 0 {
		text := strings.Join(warnings, "\n")
		reply.Warnings = &text
	}

	switch rpc {
	case "HostVolume.Create":
		args := structs.HostVolumeCreateRequest{Volume: body.Volume, PolicyOverride: body.PolicyOverride, WriteRequest: wr}
		var out structs.HostVolumeCreateResponse
		if err := s.agent.RPC(rpc, &args, &out); err != nil {
			return nil, err
		}
		reply.Volume = out.Volume
		setIndex(resp, out.Index)
	default:
		args := structs.HostVolumeRegisterRequest{Volume: body.Volume, PolicyOverride: body.PolicyOverride, WriteRequest: wr}
		var out structs.HostVolumeRegisterResponse
		if err := s.agent.RPC(rpc, &args, &out); err != nil {
			return nil, err
		}
		reply.Volume = out.Volume
		setIndex(resp, out.Index)
	}
	return reply, nil
}

// decodeHostVolumeWriteBody reads one JSON object and nothing after it. Key
// matching is case-insensitive, as in the rest of the API. It also returns the
// keys that name no field, as "Name" for the top level and "Volume.Name" for
// the volume. Nested objects are not checked.
func decodeHostVolumeWriteBody(req *http.Request) (hostVolumeWriteBody, []string, error) {
	var body hostVolumeWriteBody
	if req.Body == nil {
		return body, nil, errors.New("empty")
	}
	data, err := io.ReadAll(io.LimitReader(req.Body, hostVolumeMaxBody+1))
	if err != nil || len(data) > hostVolumeMaxBody {
		return body, nil, errors.New("unreadable")
	}

	// Unmarshal, unlike a decoder, refuses data after the object.
	if err := json.Unmarshal(data, &body); err != nil {
		return body, nil, err
	}

	// A repeated top-level key is refused: Unmarshal merges both occurrences
	// into the body, while the map below keeps only the last, so an unknown key
	// of an earlier occurrence would be applied or ignored without being named.
	if err := hostVolumeRepeatedKey(data); err != nil {
		return body, nil, err
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return body, nil, err
	}
	var unknown []string
	var rawVolume json.RawMessage
	for key, val := range top {
		switch {
		case strings.EqualFold(key, "Volume"):
			rawVolume = val
		case strings.EqualFold(key, "PolicyOverride"):
		default:
			unknown = append(unknown, key)
		}
	}
	if body.Volume != nil {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawVolume, &fields); err != nil {
			return body, nil, err
		}
		for key := range fields {
			if !hostVolumeKnownField(key) {
				unknown = append(unknown, "Volume."+key)
			}
		}
	}
	return body, unknown, nil
}

// hostVolumeRepeatedKey returns an error if a key of the top-level object of
// data appears twice. Keys are compared under Unicode case folding, as
// encoding/json matches them to field names. data has already been unmarshalled
// without error; a body that is not an object has no keys to repeat.
func hostVolumeRepeatedKey(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return nil
	}
	seen := make(map[string]struct{})
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return errors.New("object key is not a string")
		}
		folded := foldHostVolumeKey(key)
		if _, dup := seen[folded]; dup {
			return errors.New("repeated key")
		}
		seen[folded] = struct{}{}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return err
		}
	}
	return nil
}

// foldHostVolumeKey maps each rune to the smallest rune of its case-folding
// orbit, so two keys fold to the same string exactly when strings.EqualFold
// holds for them.
func foldHostVolumeKey(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		least := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			least = min(least, f)
		}
		b.WriteRune(least)
	}
	return b.String()
}

var hostVolumeFieldNames = func() []string {
	t := reflect.TypeOf(structs.DynamicHostVolume{})
	names := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		if f := t.Field(i); f.IsExported() {
			names = append(names, f.Name)
		}
	}
	return names
}()

func hostVolumeKnownField(key string) bool {
	return slices.ContainsFunc(hostVolumeFieldNames, func(n string) bool { return strings.EqualFold(n, key) })
}

// formatHostVolumeFields lists field names sorted and without duplicates. The
// names come from the request, so each is cut short and stripped of anything
// unprintable, and the list is bounded.
func formatHostVolumeFields(names []string) string {
	clean := make([]string, 0, len(names))
	for _, n := range names {
		clean = append(clean, cleanHostVolumeField(n))
	}
	slices.Sort(clean)
	clean = slices.Compact(clean)

	more := 0
	if len(clean) > hostVolumeMaxWarnedFields {
		more = len(clean) - hostVolumeMaxWarnedFields
		clean = clean[:hostVolumeMaxWarnedFields]
	}
	out := strings.Join(clean, ", ")
	if more > 0 {
		out += fmt.Sprintf(" and %d more", more)
	}
	return out
}

func cleanHostVolumeField(name string) string {
	var b strings.Builder
	n := 0
	for _, r := range name {
		if n == hostVolumeMaxWarnedName {
			b.WriteString("...")
			break
		}
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			r = '?'
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
