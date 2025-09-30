// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package cluster

import (
	"context"
	"strings"
	"time"

	"github.com/roomzin/roomzin-go/api"
	"github.com/roomzin/roomzin-go/internal/cluster"
	"github.com/roomzin/roomzin-go/internal/command"
	"github.com/roomzin/roomzin-go/types"
)

type client struct {
	handler *cluster.Handler
	cfg     *ClusterConfig
	ctx     context.Context
	cancel  context.CancelFunc
	codecs  *types.Codecs
}

func New(cfg *ClusterConfig) (api.CacheClientAPI, error) {
	if cfg == nil {
		return nil, types.RzError("cfg must not be nil", types.KindClient)
	}

	ctx, cancel := context.WithCancel(context.Background())

	icfg := &cluster.Config{
		SeedNodeIDs:       cfg.SeedNodeIDs,
		APIPort:           cfg.APIPort,
		TCPPort:           cfg.TCPPort,
		AuthToken:         cfg.AuthToken,
		Timeout:           cfg.Timeout,
		HttpTimeout:       cfg.HttpTimeout,
		KeepAlive:         cfg.KeepAlive,
		MaxActiveConns:    cfg.MaxActiveConns,
		NodeProbeInterval: 2 * time.Second,
		DiscoveryAddr:     cfg.DiscoveryAddr,
		StaticDiscovery:   cfg.StaticDiscovery,
	}

	clusterClient := cluster.NewHandler(icfg)
	clusterClient.Start(ctx)

	c := &client{
		handler: clusterClient,
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
	}

	c.handler.SetOnReconnectCallback(func() {
		c.codecs = nil
	})

	var err error
	c.codecs, err = c.fetchCodecs()
	if err != nil {
		return nil, types.RzError(err)
	}

	return c, nil
}

func (c *client) getCodecs() *types.Codecs {
	if c.codecs != nil {
		return c.codecs
	}
	c.codecs, _ = c.fetchCodecs()
	return c.codecs
}

func (c *client) fetchCodecs() (*types.Codecs, error) {
	req, err := command.BuildGetCodecsPayload()
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseGetCodecsResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) Close() error {
	c.cancel()
	return nil
}

// --------------------------------------------------
//
//	public API
//
// --------------------------------------------------

func (c *client) GetCodecs() (*types.Codecs, error) {
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

/* ----------  READ helpers (follower)  ---------- */
func (c *client) SearchProp(p types.SearchPropPayload) ([]string, error) {
	if err := p.Verify(c.getCodecs()); err != nil {
		return nil, types.RzError(err)
	}
	req, err := command.BuildSearchPropPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseSearchPropResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) SearchAvail(p types.SearchAvailPayload) ([]types.PropertyAvail, error) {
	if err := p.Verify(c.getCodecs()); err != nil {
		return nil, types.RzError(err)
	}
	req, err := command.BuildSearchAvailPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseSearchAvailResp(c.getCodecs(), resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) PropExist(propertyID string) (bool, error) {
	if strings.TrimSpace(propertyID) == "" {
		return false, types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	req, err := command.BuildPropExistPayload(propertyID)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return false, err
	}

	result, err := command.ParsePropExistResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) PropRoomExist(p types.PropRoomExistPayload) (bool, error) {
	if err := p.Verify(); err != nil {
		return false, err
	}
	req, err := command.BuildPropRoomExistPayload(p)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return false, err
	}

	result, err := command.ParsePropRoomExistResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) PropRoomList(propertyID string) ([]string, error) {
	if strings.TrimSpace(propertyID) == "" {
		return nil, types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	req, err := command.BuildPropRoomListPayload(propertyID)
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParsePropRoomListResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) PropRoomDateList(p types.PropRoomDateListPayload) ([]string, error) {
	if err := p.Verify(); err != nil {
		return nil, types.RzError(err)
	}
	req, err := command.BuildPropRoomDateListPayload(p)
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParsePropRoomDateListResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) GetPropRoomDay(p types.GetRoomDayRequest) (types.GetRoomDayResult, error) {
	if err := p.Verify(); err != nil {
		return types.GetRoomDayResult{}, err
	}
	req, err := command.BuildGetPropRoomDayPayload(p)
	if err != nil {
		return types.GetRoomDayResult{}, err
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return types.GetRoomDayResult{}, err
	}

	result, err := command.ParseGetPropRoomDayResp(c.getCodecs(), resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

/* ----------  WRITE helpers (leader)  ---------- */

func (c *client) SetProp(p types.SetPropPayload) error {
	if err := p.Verify(c.getCodecs()); err != nil {
		return types.RzError(err)
	}
	req, err := command.BuildSetPropPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseSetPropResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) SetRoomPkg(p types.SetRoomPkgPayload) error {
	if err := p.Verify(c.getCodecs()); err != nil {
		return types.RzError(err)
	}
	req, err := command.BuildSetRoomPkgPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseSetRoomPkgResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) SetRoomAvl(p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	req, err := command.BuildSetRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseSetRoomAvlResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) IncRoomAvl(p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	req, err := command.BuildIncRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseIncRoomAvlResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) DecRoomAvl(p types.UpdRoomAvlPayload) (uint8, error) {
	if err := p.Verify(); err != nil {
		return 0, types.RzError(err)
	}
	req, err := command.BuildDecRoomAvlPayload(p)
	if err != nil {
		return 0, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return 0, types.RzError(err)
	}

	result, err := command.ParseDecRoomAvlResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}

func (c *client) DelProp(propertyID string) error {
	if strings.TrimSpace(propertyID) == "" {
		return types.RzError("VALIDATION_ERROR: propertyID is required")
	}
	req, err := command.BuildDelPropPayload(propertyID)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) DelSegment(segment string) error {
	if strings.TrimSpace(segment) == "" {
		return types.RzError("VALIDATION_ERROR: segment is required")
	}
	req, err := command.BuildDelSegmentPayload(segment)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelSegmentResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) DelPropDay(p types.DelPropDayRequest) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	req, err := command.BuildDelPropDayPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropDayResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) DelPropRoom(p types.DelPropRoomPayload) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	req, err := command.BuildDelPropRoomPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelPropRoomResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

func (c *client) DelRoomDay(p types.DelRoomDayRequest) error {
	if err := p.Verify(); err != nil {
		return types.RzError(err)
	}
	req, err := command.BuildDelRoomDayPayload(p)
	if err != nil {
		return types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, true, req)
	if err != nil {
		return types.RzError(err)
	}

	err = command.ParseDelRoomDayResp(resp.Status, resp.Fields)
	if err != nil {
		return types.RzError(err)
	}
	return nil
}

/* ----------  MISC  ---------- */
func (c *client) GetSegments() ([]types.SegmentInfo, error) {
	req, err := command.BuildGetSegmentsPayload()
	if err != nil {
		return nil, types.RzError(err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.handler.Execute(ctx, false, req)
	if err != nil {
		return nil, types.RzError(err)
	}

	result, err := command.ParseGetSegmentsResp(resp.Status, resp.Fields)
	if err != nil {
		return result, types.RzError(err)
	}
	return result, nil
}
