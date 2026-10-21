package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// AcceptSkip says why a suggestion was not accepted.
type AcceptSkip struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
}

// AcceptedItem is a suggestion that became a monitor definition.
type AcceptedItem struct {
	ItemID       string `json:"item_id"`
	DefinitionID string `json:"definition_id"`
	Name         string `json:"name"`
}

// AcceptResult is what "Accept selected" did.
type AcceptResult struct {
	Created []AcceptedItem `json:"created"`
	// Tracking is the storage request that was queued for the node, when storage rows were selected.
	Tracking *NodeDiscoveryRequestView `json:"tracking,omitempty"`
	Skipped  []AcceptSkip              `json:"skipped"`
}

const (
	acceptInterval  = 30 // seconds, as in the Console's design
	acceptFailAfter = 2
)

// definitionFor maps one suggestion to a Console monitor definition.
func definitionFor(it NodeDiscoveryItem) (NodeMonitorDefinitionInput, error) {
	in := NodeMonitorDefinitionInput{IntervalSeconds: acceptInterval, FailAfter: acceptFailAfter}
	switch it.Kind {
	case "service":
		in.Name, in.CheckType, in.Params = it.Name, "systemd", map[string]any{"unit": it.Name}
	case "container":
		in.Name, in.CheckType, in.Params = it.Name, "docker", map[string]any{"container": it.Name}
	case "pm2":
		in.Name, in.CheckType, in.Params = it.Name, "pm2", map[string]any{"app": it.Name}
	case "disk":
		in.Name, in.CheckType, in.Params = it.Name, "disk", map[string]any{"path": it.Name, "threshold_percent": float64(90)}
	case "port":
		n, err := strconv.Atoi(strings.TrimPrefix(it.ID, "port:"))
		if err != nil {
			return in, fmt.Errorf("the port number is not readable from %q", it.ID)
		}
		in.Name, in.CheckType, in.Params = "port "+strconv.Itoa(n), "port", map[string]any{"port": float64(n)}
	default:
		return in, fmt.Errorf("%s suggestions cannot be accepted this way", it.Kind)
	}
	return in, nil
}

// AcceptDiscoveredItems turns suggestions from the node's latest scan into monitor definitions
// (Managed in Console). Storage rows become one track_storage request. Nothing is run by Core:
// the node fetches the definitions on its next sync and reports under the same names.
func (s *NodeService) AcceptDiscoveredItems(nodeID string, itemIDs []string, actor string) (*AcceptResult, error) {
	if len(itemIDs) == 0 || len(itemIDs) > maxRequestIDs {
		return nil, fmt.Errorf("%w: choose between 1 and %d suggestions", ErrRequestInvalid, maxRequestIDs)
	}
	view, err := s.GetNodeDiscovery(nodeID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, fmt.Errorf("%w: this node has not reported what it found yet", ErrRequestInvalid)
	}
	byID := map[string]NodeDiscoveryItem{}
	for _, it := range view.Items {
		byID[it.ID] = it
	}
	res := &AcceptResult{Created: []AcceptedItem{}, Skipped: []AcceptSkip{}}
	var storage []string
	seen := map[string]bool{}
	for _, id := range itemIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		it, ok := byID[id]
		switch {
		case !ok:
			res.Skipped = append(res.Skipped, AcceptSkip{id, "The node did not report this suggestion."})
			continue
		case it.Status == "monitored":
			res.Skipped = append(res.Skipped, AcceptSkip{id, "Something already monitors this."})
			continue
		case it.Dismissed:
			res.Skipped = append(res.Skipped, AcceptSkip{id, "This suggestion is dismissed. Restore it first."})
			continue
		case it.Kind == "log":
			res.Skipped = append(res.Skipped, AcceptSkip{id, "Log files need a log key before they can ship."})
			continue
		case it.Kind == "storage":
			storage = append(storage, it.Name)
			continue
		}
		in, err := definitionFor(it)
		if err != nil {
			res.Skipped = append(res.Skipped, AcceptSkip{id, err.Error()})
			continue
		}
		def, err := s.CreateMonitorDefinition(nodeID, in, actor)
		switch {
		case err == nil:
			res.Created = append(res.Created, AcceptedItem{ItemID: id, DefinitionID: def.ID, Name: def.Name})
		case errors.Is(err, ErrDefinitionExists):
			res.Skipped = append(res.Skipped, AcceptSkip{id, "This server already has a monitor named " + in.Name + "."})
		case errors.Is(err, ErrDefinitionInvalid):
			res.Skipped = append(res.Skipped, AcceptSkip{id, strings.TrimPrefix(err.Error(), ErrDefinitionInvalid.Error()+": ")})
		default:
			return nil, err
		}
	}
	if len(storage) > 0 {
		req, err := s.CreateDiscoveryRequest(nodeID, NodeDiscoveryRequestInput{Type: ReqTrackPath, Payload: map[string]any{"paths": toAny(storage)}}, actor)
		if err != nil {
			for _, p := range storage {
				res.Skipped = append(res.Skipped, AcceptSkip{"storage:" + p, strings.TrimPrefix(err.Error(), ErrRequestInvalid.Error()+": ")})
			}
		} else {
			res.Tracking = req
		}
	}
	return res, nil
}

func toAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
