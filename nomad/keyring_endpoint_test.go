// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"sync"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
)

// TestKeyringEndpoint_CRUD exercises the basic keyring operations
func TestKeyringEndpoint_CRUD(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	// Upsert a new key

	key, err := structs.NewRootKey(structs.EncryptionAlgorithmAES256GCM)
	require.NoError(t, err)
	id := key.Meta.KeyID
	key.Meta.SetActive()

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey:      key,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	updateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// Get and List don't need a token here because they rely on mTLS role verification
	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID:        id,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var getResp structs.KeyringGetRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	require.NoError(t, err)
	require.Equal(t, updateResp.Index, getResp.Index)
	require.Equal(t, structs.EncryptionAlgorithmAES256GCM, getResp.Key.Meta.Algorithm)

	// Make a blocking query for List and wait for an Update. Note
	// that List/Get queries don't need ACL tokens in the test server
	// because they always pass the mTLS check

	var wg sync.WaitGroup
	wg.Add(1)
	var listResp structs.KeyringListRootKeyMetaResponse

	go func() {
		defer wg.Done()
		codec := rpcClient(t, srv) // not safe to share across goroutines
		listReq := &structs.KeyringListRootKeyMetaRequest{
			QueryOptions: structs.QueryOptions{
				Region:        "global",
				MinQueryIndex: getResp.Index,
			},
		}
		err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		require.NoError(t, err)
	}()

	updateReq.RootKey.Meta.CreateTime = time.Now().UTC().UnixNano()
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// wait for the blocking query to complete and check the response
	wg.Wait()
	require.Equal(t, listResp.Index, updateResp.Index)
	require.Len(t, listResp.Keys, 2) // bootstrap + new one

	// Delete the key and verify that it's gone

	delReq := &structs.KeyringDeleteRootKeyRequest{
		KeyID:        id,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var delResp structs.KeyringDeleteRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	delReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.EqualError(t, err, "active root key cannot be deleted - call rotate first")

	// set inactive
	updateReq.RootKey.Meta.SetInactive()
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.NoError(t, err)
	require.Greater(t, delResp.Index, getResp.Index)

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)
	require.Greater(t, listResp.Index, getResp.Index)
	require.Len(t, listResp.Keys, 1) // just the bootstrap key
}

// TestKeyringEndpoint_validateUpdate exercises all the various
// validations we make for the update RPC
func TestKeyringEndpoint_InvalidUpdates(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	// Setup an existing key
	key, err := structs.NewRootKey(structs.EncryptionAlgorithmAES256GCM)
	require.NoError(t, err)
	id := key.Meta.KeyID
	key.Meta.SetActive()

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: key,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	testCases := []struct {
		key            *structs.RootKey
		expectedErrMsg string
	}{
		{
			key:            &structs.RootKey{},
			expectedErrMsg: "root key metadata is required",
		},
		{
			key:            &structs.RootKey{Meta: &structs.RootKeyMeta{}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key:            &structs.RootKey{Meta: &structs.RootKeyMeta{KeyID: "invalid"}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key: &structs.RootKey{Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmAES256GCM,
			}},
			expectedErrMsg: "root key state \"\" is invalid",
		},
		{
			key: &structs.RootKey{Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmAES256GCM,
				State:     structs.RootKeyStateActive,
			}},
			expectedErrMsg: "root key material is required",
		},

		{
			key: &structs.RootKey{
				Key: []byte{0x01},
				Meta: &structs.RootKeyMeta{
					KeyID:     id,
					Algorithm: "whatever",
					State:     structs.RootKeyStateActive,
				}},
			expectedErrMsg: "root key algorithm cannot be changed after a key is created",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expectedErrMsg, func(t *testing.T) {
			updateReq := &structs.KeyringUpdateRootKeyRequest{
				RootKey: tc.key,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					AuthToken: rootToken.SecretID,
				},
			}
			var updateResp structs.KeyringUpdateRootKeyResponse
			err := msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
			require.EqualError(t, err, tc.expectedErrMsg)
		})
	}

}

// TestKeyringEndpoint_Rotate exercises the key rotation logic
func TestKeyringEndpoint_Rotate(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	// Setup an existing key
	key, err := structs.NewRootKey(structs.EncryptionAlgorithmAES256GCM)
	require.NoError(t, err)
	key.Meta.SetActive()

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: key,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	// Rotate the key

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	rotateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.NoError(t, err)
	require.NotEqual(t, updateResp.Index, rotateResp.Index)

	newID := rotateResp.Key.KeyID

	// Verify we have a new key and the old one is inactive

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)

	require.Greater(t, listResp.Index, updateResp.Index)
	require.Len(t, listResp.Keys, 3) // bootstrap + old + new

	for _, keyMeta := range listResp.Keys {
		if keyMeta.KeyID != newID {
			require.False(t, keyMeta.Active(), "expected old keys to be inactive")
		} else {
			require.True(t, keyMeta.Active(), "expected new key to be inactive")
		}
	}

	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID: newID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var getResp structs.KeyringGetRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	require.NoError(t, err)

	gotKey := getResp.Key
	require.Len(t, gotKey.Key, 32)
}
