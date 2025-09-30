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

	"github.com/roomzin/roomzin-go/types"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildGetSegmentsPayload() ([]byte, error) {
	var buf bytes.Buffer

	cmdName := "GETSEGMENTS"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // field count = 0

	return buf.Bytes(), nil
}

func ParseGetSegmentsResp(status string, fields []protocol.Field) ([]types.SegmentInfo, error) {
	if status != "SUCCESS" {
		if len(fields) > 0 && fields[0].FieldType == 0x01 {
			return nil, fmt.Errorf("%s", string(fields[0].Data))
		}
		return nil, fmt.Errorf("RESPONSE_ERROR")
	}

	// Fields should come in pairs: segment string followed by propCount u32
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("RESPONSE_ERROR: invalid field count: expected pairs of segment and propCount")
	}

	list := make([]types.SegmentInfo, 0, len(fields)/2)

	for i := 0; i < len(fields); i += 2 {
		// First field should be segment (string type 0x01)
		if fields[i].FieldType != 0x01 {
			return nil, fmt.Errorf("RESPONSE_ERROR: expected string segment at field %d, got type %d", i, fields[i].FieldType)
		}
		segment := string(fields[i].Data)

		// Second field should be propCount (u32 type 0x03)
		if i+1 >= len(fields) {
			return nil, fmt.Errorf("RESPONSE_ERROR: missing propCount field for segment %s", segment)
		}
		if fields[i+1].FieldType != 0x03 {
			return nil, fmt.Errorf("RESPONSE_ERROR: expected u32 propCount at field %d, got type %d", i+1, fields[i+1].FieldType)
		}
		if len(fields[i+1].Data) != 4 {
			return nil, fmt.Errorf("RESPONSE_ERROR: invalid propCount length: expected 4 bytes, got %d", len(fields[i+1].Data))
		}

		propCount := binary.LittleEndian.Uint32(fields[i+1].Data)

		list = append(list, types.SegmentInfo{
			Segment:   segment,
			PropCount: propCount,
		})
	}

	return list, nil
}
