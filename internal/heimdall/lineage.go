// InstanceID Phase 9 lineage walker — final phase of the InstanceID
// system per docs/instanceid-system-v2-r60.md §13.
//
// Given a flat index of LineageRecord entries (one per InstanceID seen
// in a game) and a target InstanceID, produces an acyclic tree linking
// the target → its source (copy target) → its enabler (the ability
// instance that minted it). Heimdall replay viewer + spectator
// /api/spectator/lineage/:id consume this.
//
// Acyclicity: the walker tracks visited IDs and returns nil rather
// than re-entering a cycle. Copy chains and shape-shift loops
// (Vesuvan Shapeshifter → Lazav → back to Vesuvan) cannot infinite-
// loop the walker.

package heimdall

import (
	"fmt"
	"strings"
)

// LineageRecord is a single object's per-instance lineage as captured
// from a game tick. Spectator captureSnapshot writes one entry per
// card with a minted InstanceID into GameSnapshot.Lineage; the
// endpoint hands the map to BuildLineageTree to render.
//
// The fields mirror the JSON-tag names on PermanentSnapshot so a single
// flat record format serves both rendering surfaces.
type LineageRecord struct {
	InstanceID        string   `json:"instance_id"`
	Name              string   `json:"name,omitempty"`
	Provenance        string   `json:"provenance,omitempty"`
	SourceInstanceID  string   `json:"source_instance_id,omitempty"`
	EnablerInstanceID string   `json:"enabler_instance_id,omitempty"`
	EnablerHistory    []string `json:"enabler_history,omitempty"`
	MergedCardIDs     []string `json:"merged_card_ids,omitempty"`
}

// LineageNode is one rendered tree node. Source/Enabler are recursively
// walked for the target's lineage; Merged is populated for Mutate /
// Meld permanents (each absorbed Card contributes a sub-node).
type LineageNode struct {
	InstanceID        string         `json:"instance_id"`
	Name              string         `json:"name,omitempty"`
	Provenance        string         `json:"provenance,omitempty"`
	SourceInstanceID  string         `json:"source_instance_id,omitempty"`
	EnablerInstanceID string         `json:"enabler_instance_id,omitempty"`
	EnablerHistory    []string       `json:"enabler_history,omitempty"`
	MergedCardIDs     []string       `json:"merged_card_ids,omitempty"`
	Source            *LineageNode   `json:"source,omitempty"`
	Enabler           *LineageNode   `json:"enabler,omitempty"`
	Merged            []*LineageNode `json:"merged,omitempty"`
	Description       string         `json:"description,omitempty"`
}

// BuildLineageTree walks the lineage forest for instanceID using
// records as the lookup table. Returns nil when instanceID is unknown
// to records. Acyclic: every recursion tracks a visited set keyed by
// InstanceID; re-entering a visited ID terminates that branch.
func BuildLineageTree(records map[string]LineageRecord, instanceID string) *LineageNode {
	if instanceID == "" || len(records) == 0 {
		return nil
	}
	if _, ok := records[instanceID]; !ok {
		return nil
	}
	return walkLineage(records, instanceID, map[string]bool{})
}

func walkLineage(records map[string]LineageRecord, id string, visited map[string]bool) *LineageNode {
	if id == "" || visited[id] {
		return nil
	}
	rec, ok := records[id]
	if !ok {
		// Unknown ID: return a stub so consumers can see the dangling
		// reference rather than silently dropping it. Useful when the
		// source/enabler has aged out of the visible snapshot.
		return &LineageNode{InstanceID: id, Description: "(unknown)"}
	}
	visited[id] = true
	defer delete(visited, id)

	node := &LineageNode{
		InstanceID:        rec.InstanceID,
		Name:              rec.Name,
		Provenance:        rec.Provenance,
		SourceInstanceID:  rec.SourceInstanceID,
		EnablerInstanceID: rec.EnablerInstanceID,
		EnablerHistory:    append([]string(nil), rec.EnablerHistory...),
		MergedCardIDs:     append([]string(nil), rec.MergedCardIDs...),
	}
	if rec.SourceInstanceID != "" {
		node.Source = walkLineage(records, rec.SourceInstanceID, visited)
	}
	if rec.EnablerInstanceID != "" {
		node.Enabler = walkLineage(records, rec.EnablerInstanceID, visited)
	}
	for _, mid := range rec.MergedCardIDs {
		child := walkLineage(records, mid, visited)
		if child != nil {
			node.Merged = append(node.Merged, child)
		}
	}
	node.Description = describeNode(node)
	return node
}

// describeNode renders a human-readable one-line summary of a lineage
// node. The replay viewer + spectator endpoint both surface this.
func describeNode(node *LineageNode) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	if node.Name != "" {
		b.WriteString(node.Name)
	} else {
		b.WriteString("<unnamed>")
	}
	fmt.Fprintf(&b, " [%s]", node.InstanceID)
	if node.Provenance != "" {
		fmt.Fprintf(&b, " (%s)", node.Provenance)
	}
	return b.String()
}

// RenderLineageText flattens a LineageNode into a single multi-line
// text representation, suitable for replay-viewer tooltips and Loki
// forensic dumps. The output shape matches the design v2 §13 example:
//
//	Sai of the Shining Sword [h0OGW20042]
//	  → minted Thopter token [h0TKC11234] via ability instance [h0ABXW40456]
//
// Returns the empty string when node is nil.
func RenderLineageText(node *LineageNode) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	renderLineageLines(&b, node, 0)
	return b.String()
}

func renderLineageLines(b *strings.Builder, node *LineageNode, depth int) {
	if node == nil {
		return
	}
	if depth > 0 {
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString("→ ")
	}
	b.WriteString(node.Description)
	b.WriteString("\n")
	if node.Enabler != nil {
		b.WriteString(strings.Repeat("  ", depth+1))
		b.WriteString("via enabler: ")
		b.WriteString(node.Enabler.Description)
		b.WriteString("\n")
	}
	if node.Source != nil {
		renderLineageLines(b, node.Source, depth+1)
	}
	for _, m := range node.Merged {
		b.WriteString(strings.Repeat("  ", depth+1))
		b.WriteString("merged: ")
		b.WriteString(m.Description)
		b.WriteString("\n")
	}
}
