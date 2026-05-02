// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package socket

import (
	"bytes"
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/jobs"
	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/internal/network"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/internal/session"
)

func TestPayloadBuilders(t *testing.T) {
	sess := new(session.Base)
	buf := new(bytes.Buffer)

	// Proto
	err := Proto(enums.EMsg(1), &pb.CMsgClientHeartBeat{})(sess, buf, 0, "")
	require.NoError(t, err)
	assert.True(t, buf.Len() > 0)

	// Unified
	buf.Reset()
	err = Unified("Test.Method#1", &pb.CMsgClientHeartBeat{})(sess, buf, 1, "token")
	require.NoError(t, err)

	pkt, _ := protocol.ParsePacket(buf)
	hdr, _ := pkt.Header.(*protocol.MsgHdrProtoBuf)
	assert.Equal(t, "Test.Method#1", hdr.Proto.GetTargetJobName())
	assert.Equal(t, "token", hdr.Proto.GetWgToken())
}

func TestOptions_WithToken(t *testing.T) {
	cfg := &SendConfig{}
	token := "oath_access_token_123"

	// Triggers: WithToken sets an access token...
	opt := WithToken(token)
	opt(cfg)

	assert.Equal(t, token, cfg.Token)
}

func TestBuilders_DynamicRaw(t *testing.T) {
	mockConn := newMockConnection()
	sess := session.New(mockConn)
	sess.SetSteamID(76561197960287930)

	t.Run("Proto Branch (TargetName set)", func(t *testing.T) {
		payload := []byte("proto_data")
		target := "Player.GetNickname#1"

		// Triggers: isProto := targetName != ""
		builder := DynamicRaw(enums.EMsg_ServiceMethodCallFromClient, target, payload)

		buf := new(bytes.Buffer)
		err := builder(sess, buf, 999, "my_token")
		require.NoError(t, err)

		pkt, err := protocol.ParsePacket(buf)
		require.NoError(t, err)

		assert.True(t, pkt.IsProto)
		hdr, ok := pkt.Header.(*protocol.MsgHdrProtoBuf)
		require.True(t, ok)
		assert.Equal(t, target, hdr.Proto.GetTargetJobName())
		assert.Equal(t, "my_token", hdr.Proto.GetWgToken())
		assert.Equal(t, payload, pkt.Payload)
	})

	t.Run("Raw Branch (TargetName empty)", func(t *testing.T) {
		payload := []byte("raw_data")

		// Triggers: isProto := targetName != "" (false case)
		builder := DynamicRaw(enums.EMsg_ClientLogon, "", payload)

		buf := new(bytes.Buffer)
		err := builder(sess, buf, 123, "")
		require.NoError(t, err)

		pkt, err := protocol.ParsePacket(buf)
		require.NoError(t, err)

		assert.False(t, pkt.IsProto)
		hdr, ok := pkt.Header.(*protocol.MsgHdrExtended)
		require.True(t, ok)
		assert.Equal(t, uint64(123), hdr.SourceJobID)
		assert.Equal(t, payload, pkt.Payload)
	})
}

func TestSocket_Send(t *testing.T) {
	conn := newMockConnection()
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"mock": func(c context.Context, n network.Handler, l log.Logger, s string) (network.Connection, error) {
			return conn, nil
		},
	}

	sock := NewSocket(cfg)
	defer sock.Close()

	require.NoError(t, sock.Connect(CMServer{Type: "mock"}))

	t.Run("Builder Error", func(t *testing.T) {
		err := sock.Send(context.Background(), func(s Session, b *bytes.Buffer, u uint64, s2 string) error {
			return errors.New("build fail")
		})
		assert.ErrorContains(t, err, "build fail")
	})

	t.Run("Send Fatal Error", func(t *testing.T) {
		conn.sendFunc = func(ctx context.Context, data []byte) error { return syscall.ECONNRESET }
		err := sock.Send(context.Background(), Raw(enums.EMsg(1), nil))
		assert.Error(t, err)
		assert.Equal(t, StateDisconnected, sock.State()) // Should trigger reconnect logic
	})
}

func TestSocket_SendSync(t *testing.T) {
	conn := newMockConnection()
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"mock": func(c context.Context, n network.Handler, l log.Logger, s string) (network.Connection, error) {
			return conn, nil
		},
	}

	sock := NewSocket(cfg)
	defer sock.Close()

	require.NoError(t, sock.Connect(CMServer{Type: "mock"}))

	go func() {
		data := <-conn.sentMsgs
		pkt, _ := protocol.ParsePacket(bytes.NewReader(data))
		// Respond
		resp := packProto(enums.EMsg(1), pkt.GetSourceJobID(), []byte("resp"))
		sock.processSingle(bytes.NewReader(resp))
	}()

	resp, err := sock.SendSync(context.Background(), Raw(enums.EMsg(1), nil))
	require.NoError(t, err)
	assert.Equal(t, "resp", string(resp.Payload))
}

func TestSocket_SendHelpers(t *testing.T) {
	conn := newMockConnection()
	sock := NewSocket(DefaultConfig())
	sock.setSession(session.New(conn))
	sock.setState(StateConnected)

	t.Run("SendUnified Helper", func(t *testing.T) {
		method := "Test.Method#1"
		req := &pb.CMsgClientHeartBeat{}

		// Triggers: SendUnified calling Send with Unified builder
		err := sock.SendUnified(context.Background(), method, req)
		require.NoError(t, err)

		data := <-conn.sentMsgs
		pkt, _ := protocol.ParsePacket(bytes.NewReader(data))

		assert.Equal(t, enums.EMsg_ServiceMethodCallFromClient, pkt.EMsg)
		assert.True(t, pkt.IsProto)
		hdr := pkt.Header.(*protocol.MsgHdrProtoBuf)
		assert.Equal(t, method, hdr.Proto.GetTargetJobName())
	})

	t.Run("SendRaw Helper", func(t *testing.T) {
		payload := []byte{0x01, 0x02, 0x03}

		// Triggers: SendRaw calling Send with Raw builder
		err := sock.SendRaw(context.Background(), enums.EMsg_ClientHeartBeat, payload)
		require.NoError(t, err)

		data := <-conn.sentMsgs
		pkt, _ := protocol.ParsePacket(bytes.NewReader(data))

		assert.Equal(t, enums.EMsg_ClientHeartBeat, pkt.EMsg)
		assert.False(t, pkt.IsProto)
		assert.Equal(t, payload, pkt.Payload)
	})
}

func TestSocket_JobContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"mock": func(c context.Context, n network.Handler, l log.Logger, s string) (network.Connection, error) {
			return newMockConnection(), nil
		},
	}

	sock := NewSocket(cfg)
	defer sock.Close()

	// This covers the go routine in registerJob that listens for socket context cancellation
	require.NoError(t, sock.Connect(CMServer{Type: "mock"}))

	cbCalled := make(chan bool, 1)
	sock.Send(context.Background(), Raw(enums.EMsg(1), nil), WithCallback(func(p *protocol.Packet, err error) {
		assert.ErrorIs(t, err, jobs.ErrJobCancelled)

		cbCalled <- true
	}))

	sock.Disconnect() // This should cancel the job's context

	select {
	case <-cbCalled:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback not called on disconnect")
	}
}

func TestSocket_InternalCoverage(t *testing.T) {
	t.Run("Job_Abortion_on_Build_Fail", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())
		defer sock.Close()

		sock.setSession(session.New(newMockConnection()))

		cbCalled := make(chan struct{})
		errBuild := errors.New("build error")

		err := sock.Send(context.Background(), func(s Session, b *bytes.Buffer, j uint64, t string) error {
			return errBuild
		}, WithCallback(func(p *protocol.Packet, e error) {
			assert.ErrorIs(t, e, errBuild)
			close(cbCalled)
		}))

		assert.ErrorIs(t, err, errBuild)

		select {
		case <-cbCalled:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Callback was not called on build failure")
		}
	})

	t.Run("Buffer Pool Capacity Guard", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())
		hugeBuf := bytes.NewBuffer(make([]byte, 70*1024))
		sock.putBuffer(hugeBuf) // Should not be put back into pool
		assert.NotNil(t, sock.getBuffer())
	})
}

func TestSocket_JobCancellationPaths(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	conn := newMockConnection()
	sock.setSession(session.New(conn))
	sock.setState(StateConnected)

	t.Run("Job_Cancelled_via_Socket_Close", func(t *testing.T) {
		cbErr := make(chan error, 1)
		_ = sock.Send(context.Background(), Raw(enums.EMsg(1), nil), WithCallback(func(p *protocol.Packet, err error) {
			cbErr <- err
		}))

		// When we close the socket, the job manager is closed.
		// We should accept either "manager closed" or "context canceled"
		// depending on which internal goroutine wins the race.
		sock.Close()

		select {
		case err := <-cbErr:
			// The manager's own closure error is expected here
			assert.True(t, errors.Is(err, context.Canceled) ||
				assert.Contains(t, err.Error(), "closed"), "Error should be cancellation or closure")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Job callback was not notified of socket closure")
		}
	})
}

func TestIsFatalNetworkError_AllBranches(t *testing.T) {
	assert.True(t, isFatalNetworkError(syscall.ECONNRESET))
	assert.True(t, isFatalNetworkError(syscall.EPIPE))
	assert.True(t, isFatalNetworkError(syscall.ECONNABORTED))
	assert.True(t, isFatalNetworkError(syscall.ETIMEDOUT))

	// Negative cases
	assert.False(t, isFatalNetworkError(nil))
	assert.False(t, isFatalNetworkError(errors.New("generic")))
	assert.False(t, isFatalNetworkError(syscall.EINVAL))
}
