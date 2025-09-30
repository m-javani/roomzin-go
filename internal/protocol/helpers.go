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
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/roomzin/roomzin-go/types"
)

// / BitmaskToRateFeatureStrings converts 24-bit mask → []string (matches Rust bitmask_to_rate_feature_string)
func BitmaskToRateFeatureStrings(codecs *types.Codecs, mask uint32) []string {
	if codecs == nil || len(codecs.RateFeatures) == 0 {
		return []string{}
	}

	out := make([]string, 0, 24) // pre-allocate up to 24
	for i := 0; i < 24 && i < len(codecs.RateFeatures); i++ {
		if mask&(1<<uint(i)) != 0 {
			out = append(out, codecs.RateFeatures[i])
		}
	}
	return out
}

// u16ToDate unpacks the 16-bit packed date (same bit layout as Rust)
func U16ToDate(packed uint16) (string, error) {
	yearOffset := int((packed >> 9) & 0b111)
	month := int((packed>>5)&0b1111) + 1
	day := int(packed&0b11111) + 1

	// use current year as base (like Rust)
	baseYear := time.Now().Year()
	t := time.Date(baseYear+yearOffset, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Month() != time.Month(month) || t.Day() != day {
		return "", errors.New("invalid packed date")
	}
	return t.Format("2006-01-02"), nil
}

func MakeF64(v float64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return b
}
func MakeU64(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func MakeU32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func BytesToPropertyID(data []byte) string {
	// 1. Too short → return empty
	if len(data) < 7 {
		return ""
	}

	// 2. Short string marker
	if data[6] == 0xF0 {
		// Left segment: 0..5
		leftLen := 0
		for i := 0; i < 6; i++ {
			if i >= len(data) || data[i] == 0 {
				break
			}
			leftLen++
		}

		// Right segment: 7..15
		rightLen := 0
		for i := 7; i < len(data); i++ {
			if data[i] == 0 {
				break
			}
			rightLen++
		}

		result := make([]byte, leftLen+rightLen)
		copy(result[:leftLen], data[:leftLen])
		copy(result[leftLen:], data[7:7+rightLen])
		return string(result)
	}

	// 3. UUID detection (valid version)
	version := (data[6] & 0xF0) >> 4
	switch version {
	case 1, 2, 3, 4, 5, 7:
		var uuidBytes [16]byte
		if len(data) >= 16 {
			copy(uuidBytes[:], data[:16])
		} else {
			copy(uuidBytes[:], data) // pad remaining with zeros
		}

		if u, err := uuid.FromBytes(uuidBytes[:]); err == nil {
			return u.String()
		}
	}

	// This should never happen with proper server data
	return ""
}
