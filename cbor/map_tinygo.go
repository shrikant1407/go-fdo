// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

//go:build tinygo

package cbor

import "reflect"

func mapClear(rv reflect.Value) {
	iter := rv.MapRange()
	for iter.Next() {
		rv.SetMapIndex(iter.Key(), reflect.Value{})
	}
}
