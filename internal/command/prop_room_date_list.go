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
	"fmt"
	"slices"

	"github.com/roomzin/roomzin-go/types"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildPropRoomDateListPayload(p types.PropRoomDateListPayload) ([]byte, error) {
	var buf bytes.Buffer

	cmdName := "PROPROOMDATELIST"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	_ = binary.Write(&buf, binary.LittleEndian, uint16(2)) // two fields

	fields := []struct {
		id   uint16
		typ  byte
		data []byte
	}{
		{0x01, 0x01, []byte(p.PropertyID)},
		{0x02, 0x01, []byte(p.RoomType)},
	}
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

func ParsePropRoomDateListResp(status string, fields []protocol.Field) ([]string, error) {
	if status != "SUCCESS" {
		if len(fields) > 0 && fields[0].FieldType == 0x01 {
			return nil, fmt.Errorf("%s", string(fields[0].Data))
		}
		return nil, fmt.Errorf("RESPONSE_ERROR")
	}
	out := make([]string, 0, len(fields))
	for i := range fields {
		if s := string(fields[i].Data); s != "" {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out, nil
}
