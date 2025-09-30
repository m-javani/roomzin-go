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

	"github.com/roomzin/roomzin-go/internal/protocol"

	"errors"
)

func BuildDelSegmentPayload(segment string) ([]byte, error) {
	var buf bytes.Buffer

	cmdName := "DELSEGMENT"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))

	idBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(idBytes, 0x01)
	buf.Write(idBytes)
	buf.WriteByte(0x01)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(segment)))
	buf.WriteString(segment)

	return buf.Bytes(), nil
}

func ParseDelSegmentResp(status string, fields []protocol.Field) error {
	if status == "SUCCESS" {
		return nil
	}
	if len(fields) > 0 && fields[0].FieldType == 0x01 {
		return errors.New(string(fields[0].Data))
	}
	return errors.New("RESPONSE_ERROR")
}
