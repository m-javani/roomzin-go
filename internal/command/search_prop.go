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

func BuildSearchPropPayload(p types.SearchPropPayload) ([]byte, error) {
	var buf bytes.Buffer

	// command name
	cmdName := "SEARCHPROP"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	type fld struct {
		id   uint16
		typ  byte
		data []byte
	}
	var fields []fld

	// required first
	fields = append(fields, fld{0x01, 0x01, []byte(p.Segment)})

	// optional helpers
	if v := p.Area; v != nil {
		fields = append(fields, fld{0x02, 0x01, []byte(*v)})
	}
	if v := p.Type; v != nil {
		fields = append(fields, fld{0x03, 0x01, []byte(*v)})
	}
	if v := p.Stars; v != nil {
		fields = append(fields, fld{0x04, 0x02, []byte{*v}})
	}
	if v := p.Category; v != nil {
		fields = append(fields, fld{0x05, 0x01, []byte(*v)})
	}
	if v := p.Amenities; v != nil {
		fields = append(fields, fld{0x06, 0x01, []byte(strings.Join(*v, ","))})
	}
	if v := p.Longitude; v != nil {
		fields = append(fields, fld{0x07, 0x03, protocol.MakeF64(*v)})
	}
	if v := p.Latitude; v != nil {
		fields = append(fields, fld{0x08, 0x03, protocol.MakeF64(*v)})
	}
	if v := p.Limit; v != nil {
		fields = append(fields, fld{0x09, 0x03, protocol.MakeU64(*v)})
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
func ParseSearchPropResp(status string, fields []protocol.Field) ([]string, error) {
	if status != "SUCCESS" {
		if len(fields) > 0 && fields[0].ID == 0x01 && fields[0].FieldType == 0x01 {
			return nil, fmt.Errorf("%s", string(fields[0].Data))
		}
		return nil, fmt.Errorf("RESPONSE_ERROR")
	}
	ids := make([]string, 0, len(fields))
	for i := range fields {
		f := fields[i]
		if f.ID != uint16(i+1) {
			return nil, fmt.Errorf("RESPONSE_ERROR: invalid field ID %d: expected %d", f.ID, i+1)
		}
		if f.FieldType != 0x01 {
			return nil, fmt.Errorf("RESPONSE_ERROR: invalid field type at ID %d: expected 0x01", f.ID)
		}
		ids = append(ids, protocol.BytesToPropertyID(f.Data))
	}
	return ids, nil
}
