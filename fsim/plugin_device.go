// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package fsim

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/fido-device-onboard/go-fdo/cbor"
	"github.com/fido-device-onboard/go-fdo/serviceinfo"
)

// PluginDeviceModule adapts an executable plugin to the internal module
// interface.
type PluginDeviceModule struct {
	Plugin

	once  sync.Once
	proto *pluginProtocol
	err   error
}

var _ serviceinfo.DeviceModule = (*PluginDeviceModule)(nil)

// Transition implements serviceinfo.DeviceModule.
func (m *PluginDeviceModule) Transition(active bool) error {
	if !active {
		return nil
	}

	m.once.Do(func() {
		w, r, err := m.Start()
		if err != nil {
			m.err = err
			return
		}
		m.proto = &pluginProtocol{in: w, out: bufio.NewScanner(r)}
	})

	return m.err
}

// Receive implements serviceinfo.DeviceModule.
func (m *PluginDeviceModule) Receive(ctx context.Context, moduleName, messageName string, messageBody io.Reader, respond func(message string) io.Writer, yield func()) error {
	if m.proto == nil {
		return errors.New("plugin module not activated")
	}

	name := moduleName + ":" + messageName

	// Decode CBOR and encode to plugin protocol
	var val interface{}
	if err := cbor.NewDecoder(messageBody).Decode(&val); err != nil {
		return fmt.Errorf("error decoding message %q body: %w", name, err)
	}
	if err := m.proto.Send(dKey, base64.StdEncoding.EncodeToString([]byte(messageName))); err != nil {
		return fmt.Errorf("error sending message %q to plugin: %w", name, err)
	}
	if err := m.proto.EncodeValue(val); err != nil {
		return fmt.Errorf("error encoding message %q body: %w", name, err)
	}

	return nil
}

// Yield implements serviceinfo.DeviceModule.
func (m *PluginDeviceModule) Yield(ctx context.Context, respond func(message string) io.Writer, yield func()) error {
	if m.proto == nil {
		return errors.New("plugin module not activated")
	}

	// Send yield to plugin
	if err := m.proto.Send(dYield, nil); err != nil {
		return err
	}

	// Read messages until plugin yields
	for {
		c, param, err := m.proto.Recv()
		if err != nil {
			return err
		}

		switch c {
		case dYield:
			return nil

		case dBreak:
			yield()
			return nil

		case dKey:
			message := param.(string)
			w := respond(message)

			val, err := m.proto.DecodeValue()
			if err != nil {
				return err
			}
			return cbor.NewEncoder(w).Encode(val)

		default:
			return fmt.Errorf("invalid data: got unexpected command %q while parsing", c)
		}
	}
}
