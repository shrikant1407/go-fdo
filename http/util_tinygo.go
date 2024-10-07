// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

//go:build tinygo

package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/fido-device-onboard/go-fdo/protocol"
)

func msgTypeFromPath(w http.ResponseWriter, r *http.Request) (uint8, bool) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return 0, false
	}
	path := strings.TrimPrefix(r.URL.Path, "/fdo/101/msg/")
	if strings.Contains(path, "/") {
		w.WriteHeader(http.StatusNotFound)
		return 0, false
	}
	typ, err := strconv.ParseUint(path, 10, 8)
	if err != nil {
		writeErr(w, 0, fmt.Errorf("invalid message type"))
		return 0, false
	}
	return uint8(typ), true
}

func (h Handler) debugRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, msgType uint8, resp protocol.Responder) {
	// TODO: Implement
}

func debugRequest(req *http.Request, body *bytes.Buffer) {
	// TODO: Implement
}

func debugResponse(resp *http.Response) {
	// TODO: Implement
}
