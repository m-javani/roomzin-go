// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package command

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/roomzin/roomzin-go/types"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildSetRoomAvlPayload(p types.UpdRoomAvlPayload) ([]byte, error) {
	var buf bytes.Buffer

	// command name
	cmdName := "SETROOMAVL"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	// fields
	type fld struct {
		id   uint16
		typ  byte
		data []byte
	}
	fields := []fld{
		{0x01, 0x01, []byte(p.PropertyID)},
		{0x02, 0x01, []byte(p.RoomType)},
		{0x03, 0x01, []byte(p.Date)},
		{0x04, 0x02, []byte{p.Amount}},
	}

	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(fields)))
	for _, f := range fields {
		idBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(idBytes, f.id) // ← Write 2 bytes for ID
		buf.Write(idBytes)
		buf.WriteByte(f.typ)
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(f.data)))
		buf.Write(f.data)
	}
	return buf.Bytes(), nil
}

func ParseSetRoomAvlResp(status string, fields []protocol.Field) (uint8, error) {
	if status == "SUCCESS" {
		b := fields[0].Data
		if len(b) != 1 {
			return 0, errors.New("RESPONSE_ERROR: missing or invalid scalar value")
		}
		return b[0], nil
	}
	msgB := fields[0].Data
	return 0, fmt.Errorf("%s", string(msgB))
}
