// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package types

// GetRoomDayResult defines the result for retrieving room details for a specific date (GETPROPROOMDAY command).
type GetRoomDayResult struct {
	PropertyID   string
	Date         string
	Availability uint8
	FinalPrice   uint32
	RateFeature  []string
}

// DayAvail one day inside a property.
type DayAvail struct {
	Date         string
	Availability uint8
	FinalPrice   uint32
	RateFeature  []string
}

// PropertyAvail one property + all its days.
type PropertyAvail struct {
	PropertyID string
	Days       []DayAvail
}

type SegmentInfo struct {
	Segment   string
	PropCount uint32
}
