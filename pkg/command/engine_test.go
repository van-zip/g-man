// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package command

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCaller struct {
	id      string
	name    string
	isAdmin bool
}

func (c mockCaller) ID() string          { return c.id }
func (c mockCaller) DisplayName() string { return c.name }
func (c mockCaller) IsAdmin() bool       { return c.isAdmin }

func TestEngine_BasicRegistrationAndExecution(t *testing.T) {
	e := NewEngine()

	// Test 1: Simple CommandHandler registration
	e.Register("ping", func(ctx context.Context, args []string) (string, error) {
		return "pong", nil
	})

	res, err := e.Execute(context.Background(), "!ping")
	require.NoError(t, err)
	assert.Equal(t, "pong", res)

	// Test 2: Unknown command error
	_, err = e.Execute(context.Background(), "/unknown")
	assert.ErrorContains(t, err, "unknown command \"unknown\"")
}

func TestEngine_AdminAuthorization(t *testing.T) {
	e := NewEngine()

	e.Register("shutdown", func(ctx context.Context, args []string) (string, error) {
		return "stopping", nil
	}, WithAdmin())

	// Test 1: Executing without caller in context
	_, err := e.Execute(context.Background(), "!shutdown")
	assert.ErrorContains(t, err, "unauthorized command execution")

	// Test 2: Executing with non-admin caller
	ctxUser := WithCaller(context.Background(), mockCaller{id: "123", name: "Bob", isAdmin: false})
	_, err = e.Execute(ctxUser, "!shutdown")
	assert.ErrorContains(t, err, "unauthorized command execution")

	// Test 3: Executing with admin caller
	ctxAdmin := WithCaller(context.Background(), mockCaller{id: "456", name: "Alice", isAdmin: true})
	res, err := e.Execute(ctxAdmin, "!shutdown")
	require.NoError(t, err)
	assert.Equal(t, "stopping", res)
}

func TestEngine_DynamicReflectionRegistration(t *testing.T) {
	e := NewEngine()

	// Register dynamic custom signature with typical types
	e.Register("math", func(ctx context.Context, a int, b float64, msg string) (string, error) {
		caller, ok := CallerFromContext(ctx)
		if ok && caller.ID() == "admin" {
			return "Access granted", nil
		}

		return "", errors.New("forbidden")
	})

	// Test 1: Successful typed parsing
	ctx := WithCaller(context.Background(), mockCaller{id: "admin", name: "Alice", isAdmin: true})
	res, err := e.Execute(ctx, "math 10 2.5 hello")
	require.NoError(t, err)
	assert.Equal(t, "Access granted", res)

	// Test 2: Invalid argument types
	_, err = e.Execute(ctx, "math ten 2.5 hello")
	assert.ErrorContains(t, err, "argument <arg1> must be of type int (got \"ten\")")
}

func TestEngine_CustomTypeParser(t *testing.T) {
	e := NewEngine()

	type CustomID struct {
		Value string
	}

	// Register a type parser for CustomID
	e.RegisterTypeParser(reflect.TypeOf(CustomID{}), func(valStr string) (any, error) {
		if valStr == "invalid" {
			return nil, errors.New("cannot parse")
		}

		return CustomID{Value: valStr}, nil
	})

	e.Register("lookup", func(ctx context.Context, id CustomID) (string, error) {
		return "Found ID: " + id.Value, nil
	})

	// Test 1: Valid custom type parsing
	res, err := e.Execute(context.Background(), "!lookup my-id")
	require.NoError(t, err)
	assert.Equal(t, "Found ID: my-id", res)

	// Test 2: Custom parser error propagation
	_, err = e.Execute(context.Background(), "!lookup invalid")
	assert.ErrorContains(t, err, "argument <arg1> must be of type CustomID (got \"invalid\")")
}
