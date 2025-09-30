// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// PrependHeader takes the already-serialised payload (status string + fields)
// and returns a complete frame ready to write to the server:
// | magic(1) | clrid(4) | totalLen(4) | payload |
// totalLen == len(payload)
func PrependHeader(clrid uint32, payload []byte) []byte {
	totalLen := uint32(len(payload))
	out := make([]byte, 9+totalLen)
	out[0] = 0xFF
	binary.LittleEndian.PutUint32(out[1:5], clrid)
	binary.LittleEndian.PutUint32(out[5:9], totalLen)
	copy(out[9:], payload)
	return out
}

var (
	ErrShortFrame   = errors.New("incomplete frame")
	ErrMissingMagic = errors.New("missing magic byte")
)

// Header is the decoded fixed part of the frame.
type Header struct {
	ClrID    uint32
	Status   string // "SUCCESS" or "ERROR"
	FieldCnt uint16 // number of fields that follow
}

type Field struct {
	ID        uint16
	FieldType uint8
	Data      []byte
}

// DrainFrame reads a full frame and returns header + raw payload.
// The payload starts at [statusLen][status][fieldCount]...fields
func DrainFrame(r io.Reader) (hdr Header, payload []byte, err error) {
	var fix [9]byte
	if _, err = io.ReadFull(r, fix[:]); err != nil {
		return Header{}, nil, err
	}

	// Frame layout: [0xFF][ClrID:4][payloadLen:4]
	if fix[0] != 0xFF {
		return Header{}, nil, fmt.Errorf("bad magic byte: got 0x%02x", fix[0])
	}
	hdr.ClrID = binary.LittleEndian.Uint32(fix[1:5])
	payloadLen := binary.LittleEndian.Uint32(fix[5:9])

	payload = make([]byte, payloadLen)
	if _, err = io.ReadFull(r, payload); err != nil {
		return Header{}, nil, err
	}

	if len(payload) < 1 {
		return Header{}, nil, fmt.Errorf("short frame: no statusLen")
	}
	statusLen := int(payload[0])
	if len(payload) < 1+statusLen+2 {
		return Header{}, nil, fmt.Errorf("short frame: missing status or fieldCount")
	}

	hdr.Status = string(payload[1 : 1+statusLen])
	hdr.FieldCnt = binary.LittleEndian.Uint16(payload[1+statusLen : 1+statusLen+2])

	return hdr, payload, nil
}

// ParseFields decodes the flat field array from payload.
// The slice must start at the first field (not status).
func ParseFields(data []byte, fieldCount uint16) ([]Field, error) {
	fields := make([]Field, 0, fieldCount)
	offset := 0

	for i := 0; i < int(fieldCount); i++ {
		if offset+7 > len(data) {
			return nil, fmt.Errorf("short frame: not enough bytes for field header at field %d", i)
		}
		id := binary.LittleEndian.Uint16(data[offset : offset+2])
		fieldType := data[offset+2]
		length := binary.LittleEndian.Uint32(data[offset+3 : offset+7])
		offset += 7

		if offset+int(length) > len(data) {
			return nil, fmt.Errorf("short frame: not enough data for field payload (field %d, need %d, have %d)", i, length, len(data)-offset)
		}

		fieldData := make([]byte, length)
		copy(fieldData, data[offset:offset+int(length)])
		fields = append(fields, Field{
			ID:        id,
			FieldType: fieldType,
			Data:      fieldData,
		})
		offset += int(length)
	}

	// Rust version enforces: all fields must be consumed
	if offset != len(data) {
		return nil, fmt.Errorf("extra %d bytes after parsing fields", len(data)-offset)
	}

	return fields, nil
}
