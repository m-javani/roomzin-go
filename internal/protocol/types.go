// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package protocol

import "errors"

var (
	ErrConnClosed = errors.New("connection closed")
	ErrTimeout    = errors.New("request timed out")
)

// RawResult is what the read loop delivers to waiting calls.
type RawResult struct {
	Status string
	Fields []Field
}
