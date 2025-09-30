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
	"strings"

	"github.com/roomzin/roomzin-go/types"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildSetPropPayload(p types.SetPropPayload) ([]byte, error) {
	var buf bytes.Buffer

	// command name
	cmdName := "SETPROP"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	// amenities string
	amenityStr := strings.Join(p.Amenities, ",")

	type fld struct {
		id   uint16
		typ  byte
		data []byte
	}
	fields := []fld{
		{0x01, 0x01, []byte(p.Segment)},
		{0x02, 0x01, []byte(p.Area)},
		{0x03, 0x01, []byte(p.PropertyID)},
		{0x04, 0x01, []byte(p.PropertyType)},
		{0x05, 0x01, []byte(p.Category)},
		{0x06, 0x02, []byte{p.Stars}},
		{0x07, 0x03, protocol.MakeF64(p.Latitude)},
		{0x08, 0x03, protocol.MakeF64(p.Longitude)},
		{0x09, 0x01, []byte(amenityStr)},
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

func ParseSetPropResp(status string, fields []protocol.Field) error {
	if status == "SUCCESS" {
		return nil
	}
	return fmt.Errorf("%s", string(fields[0].Data))
}
