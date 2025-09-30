// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package api

import "github.com/roomzin/roomzin-go/types"

type CacheClientAPI interface {
	GetCodecs() (*types.Codecs, error)
	SetProp(p types.SetPropPayload) error
	SearchProp(p types.SearchPropPayload) ([]string, error)
	SearchAvail(p types.SearchAvailPayload) ([]types.PropertyAvail, error)
	SetRoomPkg(p types.SetRoomPkgPayload) error
	SetRoomAvl(p types.UpdRoomAvlPayload) (uint8, error)
	IncRoomAvl(p types.UpdRoomAvlPayload) (uint8, error)
	DecRoomAvl(p types.UpdRoomAvlPayload) (uint8, error)
	PropExist(propertyID string) (bool, error)
	PropRoomExist(p types.PropRoomExistPayload) (bool, error)
	PropRoomList(propertyID string) ([]string, error)
	PropRoomDateList(p types.PropRoomDateListPayload) ([]string, error)
	DelProp(propertyID string) error
	DelSegment(segment string) error
	DelPropDay(p types.DelPropDayRequest) error
	DelPropRoom(p types.DelPropRoomPayload) error
	DelRoomDay(p types.DelRoomDayRequest) error
	GetPropRoomDay(p types.GetRoomDayRequest) (types.GetRoomDayResult, error)
	GetSegments() ([]types.SegmentInfo, error)
	Close() error
}
