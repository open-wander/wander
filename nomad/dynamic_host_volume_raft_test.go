// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	wanderversion "github.com/open-wander/wander/version"
	"github.com/shoenig/test/must"
)

// gateMember is a server member as serf would report it. The build tag is
// always the compatibility version, as on every real server, so a member that
// lacks wander_version looks exactly like Nomad 1.6.5 or an older Wander.
func gateMember(name, region, wanderVersion string, status serf.MemberStatus) serf.Member {
	tags := map[string]string{
		"role": "nomad", "region": region, "dc": "dc1", "port": "10000",
		"build": wanderversion.ClusterCompatibilityVersion, "vsn": "1",
	}
	if wanderVersion != "" {
		tags[wanderVersionTag] = wanderVersion
	}
	return serf.Member{Name: name, Addr: net.IP{127, 0, 0, 1}, Tags: tags, Status: status}
}

func TestServersMeetMinimumWanderVersion(t *testing.T) {
	ci.Parallel(t)
	min := version.Must(version.NewVersion("0.1.2"))
	alive, failed := serf.StatusAlive, serf.StatusFailed

	cases := []struct {
		name    string
		members []serf.Member
		want    bool
	}{
		{"all tagged with the dev version", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "0.1.2-dev", alive),
			gateMember("c", "global", "0.1.2-dev", alive)}, true},
		{"release and metadata pass", []serf.Member{
			gateMember("a", "global", "0.1.2", alive), gateMember("b", "global", "0.1.2+meta", alive),
			gateMember("c", "global", "0.2.0", alive), gateMember("d", "global", "1.0.0-rc1", alive)}, true},
		{"one untagged member blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "", alive)}, false},
		{"one member at 0.1.1 blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "0.1.1", alive)}, false},
		{"a prerelease of 0.1.1 blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "0.1.1-dev", alive)}, false},
		{"an unparseable tag blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "garbage", alive)}, false},
		{"a failed member below the minimum blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "0.1.1", failed)}, false},
		{"a failed untagged member blocks", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "", failed)}, false},
		{"a failed member at the minimum passes", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "0.1.2", failed)}, true},
		{"left and leaving members are ignored", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "global", "", serf.StatusLeft),
			gateMember("c", "global", "0.0.1", serf.StatusLeaving)}, true},
		{"other regions are ignored", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive), gateMember("b", "other", "", alive),
			gateMember("c", "other", "0.0.1", failed)}, true},
		{"members that are not servers are ignored", []serf.Member{
			gateMember("a", "global", "0.1.2-dev", alive),
			{Name: "client", Tags: map[string]string{"role": "client", "region": "global"}, Status: alive}}, true},
		{"no members at all", nil, false},
		{"only other regions", []serf.Member{gateMember("a", "other", "0.1.2-dev", alive)}, false},
		{"only a left server", []serf.Member{gateMember("a", "global", "0.1.2-dev", serf.StatusLeft)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.want, ServersMeetMinimumWanderVersion(tc.members, "global", min))
		})
	}
}

func TestServer_WanderVersionTag(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()

	tags := s.serf.LocalMember().Tags
	must.Eq(t, wanderversion.GetVersion().VersionNumber(), tags[wanderVersionTag])
	// The build tag stays on the compatibility version for Nomad 1.6.5 peers.
	must.StrHasPrefix(t, wanderversion.ClusterCompatibilityVersion, tags["build"])

	// A server started without a configured version advertises no tag.
	bare, cleanupBare := TestServer(t, func(c *Config) { c.WanderVersion = "" })
	defer cleanupBare()
	_, present := bare.serf.LocalMember().Tags[wanderVersionTag]
	must.False(t, present)
}

func dhvGateRequest() *structs.DynamicHostVolumeUpsertRequest {
	return &structs.DynamicHostVolumeUpsertRequest{
		WriteRequest: structs.WriteRequest{Region: "global"},
		Volumes:      []*structs.DynamicHostVolume{dhvFSMVolume("volume-a")},
	}
}

func TestServer_DynamicHostVolumeRaftApplyGate(t *testing.T) {
	ci.Parallel(t)

	t.Run("a single current server applies the command through Raft and the FSM", func(t *testing.T) {
		s, cleanup := TestServer(t, nil)
		defer cleanup()
		testutil.WaitForLeader(t, s.RPC)

		_, index, err := s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeUpsertRequestType, dhvGateRequest())
		must.NoError(t, err)
		got, err := s.State().DynamicHostVolumeByID(nil, "default", "volume-a")
		must.NoError(t, err)
		must.NotNil(t, got)
		must.Eq(t, index, got.CreateIndex)
	})

	t.Run("a server without the tag or below the minimum blocks the command", func(t *testing.T) {
		for _, other := range []string{"", "0.1.1"} {
			t.Run(fmt.Sprintf("peer %q", other), func(t *testing.T) {
				s1, cleanup1 := TestServer(t, nil)
				defer cleanup1()
				s2, cleanup2 := TestServer(t, func(c *Config) {
					c.BootstrapExpect = 0
					c.WanderVersion = other
				})
				defer cleanup2()
				// Another region has its own Raft and never blocks.
				s3, cleanup3 := TestServer(t, func(c *Config) {
					c.BootstrapExpect = 0
					c.Region = "other"
					c.WanderVersion = ""
				})
				defer cleanup3()
				TestJoin(t, s1, s2, s3)
				testutil.WaitForLeader(t, s1.RPC)
				testutil.WaitForResult(func() (bool, error) {
					return len(s1.serf.Members()) == 3, fmt.Errorf("members: %d", len(s1.serf.Members()))
				}, func(err error) { must.NoError(t, err) })

				_, _, err := s1.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeUpsertRequestType, dhvGateRequest())
				must.ErrorIs(t, err, ErrDynamicHostVolumeVersion)
				must.StrContains(t, err.Error(), "0.1.2")
				got, getErr := s1.State().DynamicHostVolumeByID(nil, "default", "volume-a")
				must.NoError(t, getErr)
				must.Nil(t, got)
			})
		}
	})

	t.Run("only the volume commands go through the gate", func(t *testing.T) {
		s, cleanup := TestServer(t, nil)
		defer cleanup()
		_, _, err := s.dynamicHostVolumeRaftApply(structs.NodeRegisterRequestType, &structs.NodeRegisterRequest{})
		must.Error(t, err)
		must.False(t, errors.Is(err, ErrDynamicHostVolumeVersion))
	})
}
