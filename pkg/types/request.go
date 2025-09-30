// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func ValidateDate(date string) error {
	var errs []string

	// Check format YYYY-MM-DD
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, date); !matched {
		errs = append(errs, fmt.Sprintf("invalid date format: %s, expected YYYY-MM-DD", date))
	} else {
		// Parse to ensure valid date
		parsedDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid date: %s", date))
		} else {
			// Check if date is in the past
			today := time.Now().Truncate(24 * time.Hour)
			if parsedDate.Before(today) {
				errs = append(errs, fmt.Sprintf("date %s is in the past", date))
			}
			// Check if date is beyond 365 days
			oneYearFromNow := today.AddDate(1, 0, 0)
			if parsedDate.After(oneYearFromNow) {
				errs = append(errs, fmt.Sprintf("date %s is beyond 365 days from today", date))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func ValidateDates(dates []string) error {
	var errs []string
	for _, date := range dates {
		err := ValidateDate(date)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New("Date errors: " + strings.Join(errs, "; "))
	}
	return nil
}

// LoginPayload defines the payload for the AUTH command.
type LoginPayload struct {
	Token string // Static token for authentication (optional)
}

func (p LoginPayload) Verify() error {
	if p.Token == "" {
		return errors.New("VALIDATION_ERROR: token is required")
	}
	return nil
}

// SetPropPayload defines the payload for adding a new property (ADDPROP command).
type SetPropPayload struct {
	Segment      string
	Area         string
	PropertyID   string
	PropertyType string
	Category     string
	Stars        uint8
	Latitude     float64
	Longitude    float64
	Amenities    []string
}

func (p SetPropPayload) Verify(codecs *Codecs) error {
	var errs []string

	if p.Segment == "" {
		errs = append(errs, "segment is required")
	}
	if p.Area == "" {
		errs = append(errs, "area is required")
	}
	if p.PropertyID == "" {
		errs = append(errs, "propertyID is required")
	}
	if p.PropertyType == "" {
		errs = append(errs, "propertyType is required")
	}
	if p.Category == "" {
		errs = append(errs, "category is required")
	}
	if p.Stars == 0 || p.Stars > 5 {
		errs = append(errs, "stars must be between 1 and 5")
	}
	if p.Latitude < -90 || p.Latitude > 90 {
		errs = append(errs, "latitude must be between -90 and 90")
	}
	if p.Longitude < -180 || p.Longitude > 180 {
		errs = append(errs, "longitude must be between -180 and 180")
	}

	if len(errs) > 0 {
		return errors.New("VALIDATION_ERROR: " + strings.Join(errs, "; "))
	}
	return nil
}

// PropRoomExistPayload defines the payload for checking if a property has a specific room type (PROPROOMEXIST command).
type PropRoomExistPayload struct {
	PropertyID string
	RoomType   string
}

// Verify validates the PropRoomExistPayload.
func (p PropRoomExistPayload) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	if p.RoomType == "" {
		return errors.New("VALIDATION_ERROR: RoomType is required")
	}
	return nil
}

// DelPropRoomPayload defines the payload for deleting a room type from a property (DELPROPROOM command).
type DelPropRoomPayload struct {
	PropertyID string
	RoomType   string
}

// Verify validates the DelPropRoomPayload.
func (p DelPropRoomPayload) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	if p.RoomType == "" {
		return errors.New("VALIDATION_ERROR: RoomType is required")
	}
	return nil
}

// PropRoomDateListPayload defines the payload for listing dates with availability for a room type (PROPROOMDATELIST command).
type PropRoomDateListPayload struct {
	PropertyID string
	RoomType   string
}

// Verify validates the PropRoomDateListPayload.
func (p PropRoomDateListPayload) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	if p.RoomType == "" {
		return errors.New("VALIDATION_ERROR: RoomType is required")
	}
	return nil
}

// DelRoomDayRequest defines the payload for deleting a room’s data for a specific date (DELROOMDAY command).
type DelRoomDayRequest struct {
	PropertyID string
	RoomType   string
	Date       string // YYYY-MM-DD
}

// Verify validates the DelRoomDayRequest.
func (p DelRoomDayRequest) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	if p.RoomType == "" {
		return errors.New("VALIDATION_ERROR: RoomType is required")
	}
	err := ValidateDate(p.Date)
	if err != nil {
		return errors.New("VALIDATION_ERROR: " + err.Error())
	}
	return nil
}

// UpdRoomAvlPayload defines the payload for updating room availability (INCROOMAVL, DECROOMAVL, SETROOMAVL commands).
type UpdRoomAvlPayload struct {
	PropertyID string
	RoomType   string
	Date       string // YYYY-MM-DD
	Amount     uint8
}

func (p UpdRoomAvlPayload) Verify() error {
	var errs []string

	if p.PropertyID == "" {
		errs = append(errs, "propertyID is required")
	}
	if p.RoomType == "" {
		errs = append(errs, "roomType is required")
	}
	if p.Amount == 0 {
		errs = append(errs, "amount must be greater than 0")
	}

	err := ValidateDate(p.Date)
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New("VALIDATION_ERROR: " + strings.Join(errs, "; "))
	}
	return nil
}

// SetRoomPkgPayload defines the payload for setting room availability, pricing, and rate features (SETROOMPKG command).
type SetRoomPkgPayload struct {
	PropertyID   string
	RoomType     string
	Date         string // YYYY-MM-DD
	Availability *uint8
	FinalPrice   *uint32
	RateFeature  []string // Optional; empty slice if not provided
}

func (p SetRoomPkgPayload) Verify(codecs *Codecs) error {
	var errs []string

	if p.PropertyID == "" {
		errs = append(errs, "propertyID is required")
	}
	if p.RoomType == "" {
		errs = append(errs, "roomType is required")
	}

	dateErr := ValidateDate(p.Date)
	if dateErr != nil {
		errs = append(errs, dateErr.Error())
	}

	if len(p.RateFeature) > 0 {
		err := ValidateRateFeatures(codecs, p.RateFeature)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New("VALIDATION_ERROR: " + strings.Join(errs, "; "))
	}
	return nil
}

// GetRoomDayRequest defines the payload for retrieving room details for a specific date (GETPROPROOMDAY command).
type GetRoomDayRequest struct {
	PropertyID string
	RoomType   string
	Date       string // YYYY-MM-DD
}

// Verify validates the GetRoomDayRequest.
func (p GetRoomDayRequest) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	if p.RoomType == "" {
		return errors.New("VALIDATION_ERROR: RoomType is required")
	}
	err := ValidateDate(p.Date)
	if err != nil {
		return errors.New("VALIDATION_ERROR: " + err.Error())
	}
	return nil
}

type SearchPropPayload struct {
	Segment   string
	Area      *string
	Type      *string
	Stars     *uint8
	Category  *string
	Amenities *[]string
	Longitude *float64
	Latitude  *float64
	Limit     *uint64
}

func (p SearchPropPayload) Verify(codecs *Codecs) error {
	var errs []string

	if p.Segment == "" {
		errs = append(errs, "segment is required")
	}
	if p.Stars != nil && (*p.Stars == 0 || *p.Stars > 5) {
		errs = append(errs, "stars must be 1–5")
	}
	if p.Latitude != nil && (*p.Latitude < -90 || *p.Latitude > 90) {
		errs = append(errs, "latitude must be between -90 and 90")
	}
	if p.Longitude != nil && (*p.Longitude < -180 || *p.Longitude > 180) {
		errs = append(errs, "longitude must be between -180 and 180")
	}

	if len(errs) > 0 {
		return errors.New("VALIDATION_ERROR: " + strings.Join(errs, "; "))
	}
	return nil
}

type SearchAvailPayload struct {
	Segment      string
	RoomType     string
	Area         *string
	PropertyID   *string
	Type         *string
	Stars        *uint8
	Category     *string
	Amenities    []string
	Longitude    *float64
	Latitude     *float64
	Date         []string
	Availability *uint8
	FinalPrice   *uint32
	RateFeature  []string
	Limit        *uint64
}

func (p SearchAvailPayload) Verify(codecs *Codecs) error {
	var errs []string

	if p.Segment == "" {
		errs = append(errs, "segment is required")
	}
	if p.RoomType == "" {
		errs = append(errs, "roomType is required")
	}
	if p.Latitude != nil {
		if *p.Latitude < -90 || *p.Latitude > 90 {
			errs = append(errs, "latitude must be between -90 and 90")
		}
	}
	if p.Longitude != nil {
		if *p.Longitude < -180 || *p.Longitude > 180 {
			errs = append(errs, "longitude must be between -180 and 180")
		}
	}
	if len(p.Date) == 0 {
		errs = append(errs, "at least one date is required")
	} else {
		if err := ValidateDates(p.Date); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(p.RateFeature) > 0 {
		if err := ValidateRateFeatures(codecs, p.RateFeature); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if p.Limit != nil && *p.Limit == 0 {
		errs = append(errs, "limit must be greater than 0")
	}

	if len(errs) > 0 {
		return errors.New("VALIDATION_ERROR: " + strings.Join(errs, "; "))
	}
	return nil
}

// DelPropDayRequest defines the payload for deleting all room data for a property on a specific date (DELPROPDAY command).
type DelPropDayRequest struct {
	PropertyID string
	Date       string // YYYY-MM-DD
}

func (p DelPropDayRequest) Verify() error {
	if p.PropertyID == "" {
		return errors.New("VALIDATION_ERROR: propertyID is required")
	}
	err := ValidateDate(p.Date)
	if err != nil {
		return errors.New("VALIDATION_ERROR: " + err.Error())
	}
	return nil
}
