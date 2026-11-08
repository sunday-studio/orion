package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"
)

var (
	// ErrNodeCannotSelfUpdate means the node has not said it can update itself.
	ErrNodeCannotSelfUpdate = errors.New("this node cannot update itself from the Console")
	// ErrNodeUpToDate means the node already runs the release, or a newer one.
	ErrNodeUpToDate = errors.New("this node is already on that release or a newer one")
	// ErrUpdateInFlight means an update for this node is already waiting or running.
	ErrUpdateInFlight = errors.New("an update for this node is already waiting")
)

// NodeUpdateTarget is the release a node should move to, with the checksum of its file for that
// node's operating system and architecture as published with the release.
type NodeUpdateTarget struct {
	Version string
	SHA256  string
}

// NodeAssetName is the release file for a node's platform.
func NodeAssetName(os, arch string) (string, error) {
	switch strings.ToLower(os) {
	case "linux", "darwin":
	default:
		return "", fmt.Errorf("nodes on %q cannot be updated from the Console", os)
	}
	switch strings.ToLower(arch) {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("nodes on %q cannot be updated from the Console", arch)
	}
	return "orion-node-" + strings.ToLower(os) + "-" + strings.ToLower(arch), nil
}

// NodeVersion is the release the node last reported.
func (s *NodeService) NodeVersion(nodeID string) string {
	var report db.NodeReport
	if err := s.db.Select("node_version").Where("node_id = ?", nodeID).Order("created_at DESC").First(&report).Error; err != nil {
		return ""
	}
	return report.NodeVersion
}

// RequestNodeUpdate asks one node to update itself. The node, not Core, downloads the release; Core
// only names the version and the checksum the node's file must have.
func (s *NodeService) RequestNodeUpdate(nodeID string, target NodeUpdateTarget, actor string) (*NodeDiscoveryRequestView, error) {
	var node db.Node
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
		return nil, ErrNodeNotFound
	}
	if !s.nodeHasCapability(nodeID, "self_update") {
		return nil, ErrNodeCannotSelfUpdate
	}
	if cmp, ok := CompareVersions(s.NodeVersion(nodeID), target.Version); !ok || cmp >= 0 {
		return nil, ErrNodeUpToDate
	}
	var open int64
	s.expireRequests(nodeID, time.Now().UTC())
	s.db.Model(&db.NodeDiscoveryRequest{}).Where("node_id = ? AND type = ? AND status IN ?", nodeID, ReqUpdateNode, []string{ReqQueued, ReqDelivered}).Count(&open)
	if open > 0 {
		return nil, ErrUpdateInFlight
	}
	return s.createRequest(nodeID, NodeDiscoveryRequestInput{Type: ReqUpdateNode, Payload: map[string]any{
		"version": target.Version, "tag": "v" + target.Version, "sha256": target.SHA256,
	}}, actor)
}
