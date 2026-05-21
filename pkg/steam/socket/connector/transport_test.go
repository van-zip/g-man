// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package connector_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/steam/socket/connector"
)

func TestSteamFramer_WriteFrame(t *testing.T) {
	framer := connector.SteamFramer{}

	t.Run("Oversized payload", func(t *testing.T) {
		err := framer.WriteFrame(io.Discard, make([]byte, 11*1024*1024))
		assert.ErrorContains(t, err, "exceeds maximum packet size")
	})

	t.Run("Valid payload on generic writer", func(t *testing.T) {
		buf := &bytes.Buffer{}
		data := []byte("hello")
		err := framer.WriteFrame(buf, data)
		require.NoError(t, err)

		out := buf.Bytes()
		assert.Equal(t, 13, len(out)) // 8 header + 5 data
		assert.Equal(t, uint32(5), binary.LittleEndian.Uint32(out[0:4]))
		assert.Equal(t, "VT01", string(out[4:8]))
		assert.Equal(t, "hello", string(out[8:]))
	})

	t.Run("Valid payload on net.Conn", func(t *testing.T) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		go func() {
			_ = framer.WriteFrame(client, []byte("pipe"))
		}()

		out := make([]byte, 12)

		server.SetReadDeadline(time.Now().Add(time.Second))
		n, err := io.ReadFull(server, out)
		require.NoError(t, err)

		assert.Equal(t, 12, n)
		assert.Equal(t, "VT01", string(out[4:8]))
		assert.Equal(t, "pipe", string(out[8:]))
	})
}

func TestSteamFramer_ReadFrame(t *testing.T) {
	framer := connector.SteamFramer{}

	t.Run("Invalid magic", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0, 0, 0, 0, 'B', 'A', 'A', 'D'})
		_, err := framer.ReadFrame(buf)
		assert.ErrorContains(t, err, "invalid magic bytes")
	})

	t.Run("Oversized packet", func(t *testing.T) {
		header := make([]byte, 8)
		binary.LittleEndian.PutUint32(header[0:4], 11*1024*1024)
		copy(header[4:8], "VT01")
		buf := bytes.NewBuffer(header)

		_, err := framer.ReadFrame(buf)
		assert.ErrorContains(t, err, "packet too large")
	})

	t.Run("Incomplete payload", func(t *testing.T) {
		header := make([]byte, 8)
		binary.LittleEndian.PutUint32(header[0:4], 100)
		copy(header[4:8], "VT01")

		buf := bytes.NewBuffer(header)
		buf.Write([]byte{1}) // Only 1 byte out of 100

		_, err := framer.ReadFrame(buf)
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("Valid frame", func(t *testing.T) {
		header := make([]byte, 8)
		binary.LittleEndian.PutUint32(header[0:4], 4)
		copy(header[4:8], "VT01")

		buf := bytes.NewBuffer(header)
		buf.Write([]byte("data"))

		payload, err := framer.ReadFrame(buf)
		require.NoError(t, err)
		assert.Equal(t, []byte("data"), payload)
	})
}

func TestSteamCipher(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	cipher := connector.NewSteamCipher(key)

	t.Run("Encrypt and Decrypt", func(t *testing.T) {
		data := []byte("secret message")
		encrypted, err := cipher.Encrypt(data)
		require.NoError(t, err)
		assert.NotEqual(t, data, encrypted)

		decrypted, err := cipher.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, data, decrypted)
	})

	t.Run("Decrypt invalid data", func(t *testing.T) {
		_, err := cipher.Decrypt([]byte("garbage_data_that_is_not_encrypted_correctly"))
		assert.Error(t, err)
	})
}
