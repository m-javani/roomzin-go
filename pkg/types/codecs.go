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
	"slices"
	"strings"
)

type Status string

type Codecs struct {
	RateFeatures []string `yaml:"rate_features"`
}

func ValidateRateFeatures(codecs *Codecs, input []string) error {
	var invalid []string
	for _, rate := range input {
		if !slices.Contains(codecs.RateFeatures, rate) {
			invalid = append(invalid, rate)
		}
	}
	if len(invalid) > 0 {
		return errors.New("Invalid rate features: " + strings.Join(invalid, ", "))
	}
	return nil
}
