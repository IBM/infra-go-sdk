package svc

import (
	"context"
	"fmt"
)

type FlashCopyMapping struct {
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	ConsistGrp  string `json:"consistgrp,omitempty"`
	AutoDelete  bool   `json:"autodelete,omitempty"`
	GrainSize   *int   `json:"grainsize,omitempty"`
	Incremental bool   `json:"incremental,omitempty"`
	CopyRate    *int   `json:"copyrate,omitempty"`
	CleanRate   *int   `json:"cleanrate,omitempty"`
	KeepTarget  bool   `json:"keeptarget,omitempty"`
	IOGrp       string `json:"iogrp,omitempty"`
}

func (c *Client) Mkfcmap(ctx context.Context, reqBody FlashCopyMapping) error {
	if reqBody.Source == "" || reqBody.Target == "" {
		return fmt.Errorf("source and target are required")
	}

	payload := make(map[string]interface{})
	if reqBody.Name != "" {
		payload["name"] = reqBody.Name
	}
	payload["source"] = reqBody.Source
	payload["target"] = reqBody.Target
	if reqBody.ConsistGrp != "" {
		payload["consistgrp"] = reqBody.ConsistGrp
	}
	if reqBody.AutoDelete {
		payload["autodelete"] = true
	}
	if reqBody.GrainSize != nil {
		payload["grainsize"] = *reqBody.GrainSize
	}
	if reqBody.Incremental {
		payload["incremental"] = true
	}
	if reqBody.CopyRate != nil {
		payload["copyrate"] = *reqBody.CopyRate
	}
	if reqBody.CleanRate != nil {
		payload["cleanrate"] = *reqBody.CleanRate
	}
	if reqBody.KeepTarget {
		payload["keeptarget"] = true
	}
	if reqBody.IOGrp != "" {
		payload["iogrp"] = reqBody.IOGrp
	}

	_, err := c.post(ctx,"mkfcmap", payload)
	if err != nil {
		decodedErr := decodeIBMError(err)
		return fmt.Errorf("failed to create FlashCopy mapping: %w", decodedErr)
	}

	return nil
}
