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
	"strings"

	"github.com/roomzin/roomzin-go/types"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

func BuildSearchAvailPayload(p types.SearchAvailPayload) ([]byte, error) {
	var buf bytes.Buffer

	// command name
	cmdName := "SEARCHAVAIL"
	buf.WriteByte(byte(len(cmdName)))
	buf.WriteString(cmdName)

	type fld struct {
		id   uint16
		typ  byte
		data []byte
	}
	var fields []fld

	// required first
	fields = append(fields,
		fld{0x01, 0x01, []byte(p.Segment)},
		fld{0x02, 0x01, []byte(p.RoomType)},
	)

	// optional helpers
	if v := p.Area; v != nil {
		fields = append(fields, fld{0x03, 0x01, []byte(*v)})
	}
	if v := p.PropertyID; v != nil {
		fields = append(fields, fld{0x04, 0x01, []byte(*v)})
	}
	if v := p.Type; v != nil {
		fields = append(fields, fld{0x05, 0x01, []byte(*v)})
	}
	if v := p.Stars; v != nil {
		fields = append(fields, fld{0x06, 0x02, []byte{*v}})
	}
	if v := p.Category; v != nil {
		fields = append(fields, fld{0x07, 0x01, []byte(*v)})
	}
	if len(p.Amenities) > 0 {
		fields = append(fields, fld{0x08, 0x01, []byte(strings.Join(p.Amenities, ","))})
	}
	if v := p.Longitude; v != nil {
		fields = append(fields, fld{0x09, 0x03, protocol.MakeF64(*v)})
	}
	if v := p.Latitude; v != nil {
		fields = append(fields, fld{0x0A, 0x03, protocol.MakeF64(*v)})
	}
	if len(p.Date) > 0 {
		fields = append(fields, fld{0x0B, 0x01, []byte(strings.Join(p.Date, ","))})
	}
	if v := p.Availability; v != nil {
		fields = append(fields, fld{0x0C, 0x02, []byte{*v}})
	}
	if v := p.FinalPrice; v != nil {
		fields = append(fields, fld{0x0D, 0x03, protocol.MakeU32(*v)})
	}
	if len(p.RateFeature) > 0 {
		fields = append(fields, fld{0x0E, 0x01, []byte(strings.Join(p.RateFeature, ","))})
	}
	if v := p.Limit; v != nil {
		fields = append(fields, fld{0x0F, 0x03, protocol.MakeU64(*v)})
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

func ParseSearchAvailResp(codecs *types.Codecs, status string, fields []protocol.Field) ([]types.PropertyAvail, error) {
	if status != "SUCCESS" {
		if len(fields) > 0 && fields[0].FieldType == 0x01 {
			return nil, fmt.Errorf("%s", string(fields[0].Data))
		}
		return nil, errors.New("RESPONSE_ERROR")
	}

	numDaysField := fields[0]
	if numDaysField.ID != 1 || numDaysField.FieldType != 0x02 || len(numDaysField.Data) != 2 {
		return nil, fmt.Errorf("RESPONSE_ERROR: expected num_days field (id=1, type=0x02, len=2)")
	}
	numDays := binary.LittleEndian.Uint16(numDaysField.Data)

	out := make([]types.PropertyAvail, 0)
	idx := 1

	for idx < len(fields) {
		f := fields[idx]

		if f.FieldType != 0x01 {
			return nil, fmt.Errorf("RESPONSE_ERROR: expected property field at index=%d, got type=0x%02x id=%d",
				idx, f.FieldType, f.ID)
		}

		propID := protocol.BytesToPropertyID(f.Data)
		idx++

		if idx >= len(fields) {
			return nil, fmt.Errorf("RESPONSE_ERROR: property %q missing days data", propID)
		}

		daysField := fields[idx]
		if daysField.FieldType != 0x08 {
			return nil, fmt.Errorf("RESPONSE_ERROR: expected days vector field for property %q, got type=0x%02x",
				propID, daysField.FieldType)
		}
		idx++

		data := daysField.Data
		if len(data) < 2 {
			return nil, fmt.Errorf("RESPONSE_ERROR: property %q days vector too short", propID)
		}

		daysCount := binary.LittleEndian.Uint16(data[0:2])
		if daysCount != numDays {
			return nil, fmt.Errorf("RESPONSE_ERROR: property %q days count mismatch: expected %d, got %d",
				propID, numDays, daysCount)
		}

		expectedDataLen := 2 + (11 * int(daysCount))
		if len(data) != expectedDataLen {
			return nil, fmt.Errorf("RESPONSE_ERROR: property %q days vector length mismatch: expected %d, got %d",
				propID, expectedDataLen, len(data))
		}

		days := make([]types.DayAvail, 0, daysCount)
		dataCursor := 2

		for d := 0; d < int(daysCount); d++ {
			if dataCursor+11 > len(data) {
				return nil, fmt.Errorf("RESPONSE_ERROR: property %q day %d data truncated", propID, d)
			}

			datePacked := binary.LittleEndian.Uint16(data[dataCursor : dataCursor+2])
			dataCursor += 2

			availability := data[dataCursor]
			dataCursor += 1

			finalPrice := binary.LittleEndian.Uint32(data[dataCursor : dataCursor+4])
			dataCursor += 4

			rateFeature := binary.LittleEndian.Uint32(data[dataCursor : dataCursor+4])
			dataCursor += 4

			dateStr, err := protocol.U16ToDate(datePacked)
			if err != nil {
				return nil, fmt.Errorf("RESPONSE_ERROR: invalid date for property=%q: %w", propID, err)
			}

			days = append(days, types.DayAvail{
				Date:         dateStr,
				Availability: availability,
				FinalPrice:   finalPrice,
				RateFeature:  protocol.BitmaskToRateFeatureStrings(codecs, rateFeature),
			})
		}

		out = append(out, types.PropertyAvail{
			PropertyID: propID,
			Days:       days,
		})
	}

	if idx != len(fields) {
		return nil, fmt.Errorf("RESPONSE_ERROR: extra fields after parsing: consumed=%d total=%d",
			idx, len(fields))
	}

	return out, nil
}
