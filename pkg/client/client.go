// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package client

import (
	"context"
	"strings"

	"github.com/m-javani/roomzin-go/internal/client"
	"github.com/m-javani/roomzin-go/internal/command"
	"github.com/m-javani/roomzin-go/pkg/types"
)

// Client is the unified Roomzin client supporting both standalone and cluster modes
type Client struct {
	handler *client.Handler
	cfg     *Config
	ctx     context.Context
	cancel  context.CancelFunc
	codecs  *types.Codecs
}

// New creates a new Roomzin client with the given configuration
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, types.RzError("cfg must not be nil", types.KindClient)
	}

	ctx, cancel := context.WithCancel(context.Background())

	icfg := &client.Config{
		Addr:      cfg.Addr,
		Port:      cfg.Port,
		Timeout:   cfg.Timeout,
		KeepAlive: cfg.KeepAlive,
		Mode:      client.ConnectionMode(cfg.Mode),
	}

	handler, err := client.NewHandler(icfg, ctx)
	if err != nil {
		cancel()
		return nil, types.RzError(err)
	}

	c := &Client{
		handler: handler,
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
	}

	c.handler.OnReconnect = func() {
		c.codecs = nil
	}

	c.codecs, err = c.fetchCodecs()
	if err != nil {
		return nil, types.RzError(err)
	}

	return c, nil
}

func (c *Client) getCodecs() *types.Codecs {
	if c.codecs != nil {
		return c.codecs
	}
	c.codecs, _ = c.fetchCodecs()
	return c.codecs
}

const codecSegment = "__codecs__"

func (c *Client) fetchCodecs() (*types.Codecs, error) {
	payload, _ := command.BuildGetCodecsPayload()
	res, err := c.handler.Execute(c.ctx, codecSegment, false, payload)
	if err != nil {
		return nil, types.RzError(err)
	}
	result, err := command.ParseGetCodecsResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	c.cancel()
	return c.handler.Close()
}

// GetCodecs returns the codecs used for encoding/decoding
func (c *Client) GetCodecs() (*types.Codecs, error) {
	if c.codecs != nil {
		return c.codecs, nil
	}
	var err error
	c.codecs, err = c.fetchCodecs()
	if err != nil {
		return nil, types.RzError(err)
	}
	return c.codecs, nil
}

// --------------------------------------------
// WRITE COMMANDS (routed to leader in cluster mode)
// --------------------------------------------

// SetProp adds or updates a property
func (c *Client) SetProp(ctx context.Context, segment string, p types.SetPropPayload) error {
	if err := p.Verify(c.getCodecs()); err != nil {
		return types.RzError(err)
	}
	payload, err := command.BuildSetPropPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseSetPropResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// SetRoomPkg sets availability and price for a room type on a specific date
func (c *Client) SetRoomPkg(ctx context.Context, segment string, p types.SetRoomPkgPayload) error {
	if err := p.Verify(c.getCodecs()); err != nil {
		return types.RzError(err)
	}
	payload, err := command.BuildSetRoomPkgPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseSetRoomPkgResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// SetRoomAvl sets availability for a room type on a specific date
func (c *Client) SetRoomAvl(ctx context.Context, segment string, p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	payload, err := command.BuildSetRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseSetRoomAvlResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// IncRoomAvl increments availability for a room type on a specific date
func (c *Client) IncRoomAvl(ctx context.Context, segment string, p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	payload, err := command.BuildIncRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseIncRoomAvlResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// DecRoomAvl decrements availability for a room type on a specific date
func (c *Client) DecRoomAvl(ctx context.Context, segment string, p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	payload, err := command.BuildDecRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseDecRoomAvlResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// DelProp deletes a property and all its data
func (c *Client) DelProp(ctx context.Context, segment string, propertyID string) error {
	if strings.TrimSpace(propertyID) == "" {
		return types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	payload, err := command.BuildDelPropPayload(propertyID)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// DelSegment deletes an entire segment
func (c *Client) DelSegment(ctx context.Context, segment string) error {
	if strings.TrimSpace(segment) == "" {
		return types.RzError("VALIDATION_ERROR: segment is required")
	}
	payload, err := command.BuildDelSegmentPayload(segment)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelSegmentResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// DelPropDay deletes a single date from all room types in a property
func (c *Client) DelPropDay(ctx context.Context, segment string, p types.DelPropDayRequest) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	payload, err := command.BuildDelPropDayPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropDayResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// DelPropRoom deletes a room type from a property
func (c *Client) DelPropRoom(ctx context.Context, segment string, p types.DelPropRoomPayload) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	payload, err := command.BuildDelPropRoomPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropRoomResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// DelRoomDay deletes a single date from a specific room type
func (c *Client) DelRoomDay(ctx context.Context, segment string, p types.DelRoomDayRequest) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	payload, err := command.BuildDelRoomDayPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, true, payload)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelRoomDayResp(res.Status, res.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

// --------------------------------------------
// READ COMMANDS (load-balanced to followers in cluster mode)
// --------------------------------------------

// SearchProp searches for properties by metadata
func (c *Client) SearchProp(ctx context.Context, segment string, p types.SearchPropPayload) ([]string, error) {
	if err := p.Verify(c.getCodecs()); err != nil {
		return nil, types.RzError(err)
	}
	payload, err := command.BuildSearchPropPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseSearchPropResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// SearchAvail searches for available properties with filters
func (c *Client) SearchAvail(ctx context.Context, segment string, p types.SearchAvailPayload) ([]types.PropertyAvail, error) {
	if err := p.Verify(c.getCodecs()); err != nil {
		return nil, types.RzError(err)
	}
	payload, err := command.BuildSearchAvailPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseSearchAvailResp(c.getCodecs(), res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// PropExist checks if a property exists
func (c *Client) PropExist(ctx context.Context, segment string, propertyID string) (bool, error) {
	if strings.TrimSpace(propertyID) == "" {
		return false, types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	payload, err := command.BuildPropExistPayload(propertyID)
	if err != nil {
		return false, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return false, types.RzError(err)
	}

	result, err := command.ParsePropExistResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// PropRoomExist checks if a room type exists in a property
func (c *Client) PropRoomExist(ctx context.Context, segment string, p types.PropRoomExistPayload) (bool, error) {
	if err := p.Verify(); err != nil {
		return false, types.RzError(err)
	}
	payload, err := command.BuildPropRoomExistPayload(p)
	if err != nil {
		return false, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return false, types.RzError(err)
	}

	result, err := command.ParsePropRoomExistResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// PropRoomList returns all room types for a property
func (c *Client) PropRoomList(ctx context.Context, segment string, propertyID string) ([]string, error) {
	if strings.TrimSpace(propertyID) == "" {
		return nil, types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	payload, err := command.BuildPropRoomListPayload(propertyID)
	if err != nil {
		return nil, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParsePropRoomListResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// PropRoomDateList returns all dates for a room type in a property
func (c *Client) PropRoomDateList(ctx context.Context, segment string, p types.PropRoomDateListPayload) ([]string, error) {
	if err := p.Verify(); err != nil {
		return nil, types.RzError(err)
	}
	payload, err := command.BuildPropRoomDateListPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParsePropRoomDateListResp(res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

// GetPropRoomDay returns availability and price for a specific room/date
func (c *Client) GetPropRoomDay(ctx context.Context, segment string, p types.GetRoomDayRequest) (types.GetRoomDayResult, error) {
	if err := p.Verify(); err != nil {
		return types.GetRoomDayResult{}, types.RzError(err)
	}
	payload, err := command.BuildGetPropRoomDayPayload(p)
	if err != nil {
		return types.GetRoomDayResult{}, types.RzError(err)
	}

	res, err := c.handler.Execute(ctx, segment, false, payload)
	if err != nil {
		return types.GetRoomDayResult{}, types.RzError(err)
	}

	result, err := command.ParseGetPropRoomDayResp(c.getCodecs(), res.Status, res.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}
