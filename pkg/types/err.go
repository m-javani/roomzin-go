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
	"strings"
)

// ---------- public error types ----------

type ErrorKind uint8

const (
	KindClient   ErrorKind = iota // SDK misuse / auth / config
	KindRequest                   // validation, not-found, overflow …
	KindInternal                  // bug, parse failure, protocol mismatch
	KindRetry                     // 429, 503, 308, role change …
)

// RoomzinError satisfies error and gives access to the kind.
type RoomzinError struct {
	Kind ErrorKind
	Code string // original code, e.g. "AUTH_ERROR"
	Msg  string // human message without prefix
}

func (e *RoomzinError) Error() string { return fmt.Sprintf("%s:%s", e.Code, e.Msg) }

// ---------- helpers for users ----------

func IsClient(err error) bool   { return isKind(err, KindClient) }
func IsRequest(err error) bool  { return isKind(err, KindRequest) }
func IsInternal(err error) bool { return isKind(err, KindInternal) }
func IsCluster(err error) bool  { return isKind(err, KindRetry) }

// errors.Is support
func (e *RoomzinError) Is(target error) bool {
	t, ok := target.(*RoomzinError)
	return ok && t.Kind == e.Kind && t.Code == e.Code
}

// ---------- internal: wrap anything that comes from the wire ----------
// RzError is the single internal entry-point.
// It accepts either:
//   - a plain Go error produced inside the SDK
//   - a string that came from the server
//
// and always returns *Error.
// RzError turns anything into *Error.
// If the caller already knows the bucket, pass it; otherwise we guess.
func RzError(in any, kind ...ErrorKind) *RoomzinError {
	if in == nil {
		return nil
	}

	// 1. already wrapped?
	var e *RoomzinError
	if errors.As(anyToError(in), &e) {
		return e
	}

	// 2. explicit bucket?
	if len(kind) > 0 {
		return &RoomzinError{
			Kind: kind[0],
			Code: codeOf(kind[0]),
			Msg:  extractMsg(in),
		}
	}

	// 3. error?
	if err, ok := in.(error); ok {
		s := err.Error()
		code, msg, _ := strings.Cut(s, ":")
		return classify(code, msg)
	}

	// 3. server string?
	if s, ok := in.(string); ok {
		code, msg, _ := strings.Cut(s, ":")
		return classify(code, msg)
	}

	return &RoomzinError{Kind: KindInternal, Code: codeOf(KindInternal), Msg: fmt.Sprint(in)}
}

// ---------- helper ----------
func extractMsg(v any) string {
	if err, ok := v.(error); ok {
		return err.Error() // <-- explicit use of .Error()
	}
	return fmt.Sprint(v)
}

func codeOf(k ErrorKind) string {
	switch k {
	case KindClient:
		return "CLIENT_ERROR"
	case KindRequest:
		return "REQUEST_ERROR"
	case KindRetry:
		return "RETRY_ERROR"
	default:
		return "INTERNAL_ERROR"
	}
}

// anyToError is a tiny helper that turns any into error.
func anyToError(v any) error {
	switch t := v.(type) {
	case error:
		return t
	case string:
		return errors.New(t)
	default:
		return fmt.Errorf("%v", v)
	}
}

func classify(code, msg string) *RoomzinError {
	switch code {
	case "AUTH_ERROR":
		return &RoomzinError{Kind: KindClient, Code: code, Msg: msg}
	case "VALIDATION_ERROR", "NOT_FOUND", "OVERFLOW", "UNDERFLOW", "FORBIDDEN":
		return &RoomzinError{Kind: KindRequest, Code: code, Msg: msg}
	case "503", "429", "308", "405":
		return &RoomzinError{Kind: KindRetry, Code: code, Msg: msg}
	default:
		// "PARSE_ERROR" , "RESPONSE_ERROR"
		return &RoomzinError{Kind: KindInternal, Code: code, Msg: msg}
	}
}

func isKind(err error, want ErrorKind) bool {
	var e *RoomzinError
	return errors.As(err, &e) && e.Kind == want
}
