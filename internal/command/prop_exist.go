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

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildPropExistPayload(propertyID string) ([]byte, error) {
	var buf bytes.Buffer

	cmdName := "PROPEXIST"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // one field

	idBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(idBytes, 0x01)
	buf.Write(idBytes)
	buf.WriteByte(0x01) // type string
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(propertyID)))
	buf.WriteString(propertyID)

	return buf.Bytes(), nil
}

func ParsePropExistResp(status string, fields []protocol.Field) (bool, error) {
	if status == "SUCCESS" {
		return fields[0].Data[0] == 1, nil
	}
	// only this path is an error
	return false, fmt.Errorf("%s", string(fields[0].Data))
}
