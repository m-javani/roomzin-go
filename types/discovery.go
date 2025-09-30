// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package types

type NodeAddr struct {
	NodeID  string `json:"node_id"`
	Host    string `json:"addr"`
	TcpPort int    `json:"tcp_port,omitempty"`
	ApiPort int    `json:"api_port,omitempty"`
}
