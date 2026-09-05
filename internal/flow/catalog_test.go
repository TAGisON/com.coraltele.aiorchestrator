package flow

import "testing"

func TestCatalogMatchesValidateSets(t *testing.T) {
	c := Catalog()
	if c.SchemaID != SchemaID {
		t.Fatalf("schema_id %q", c.SchemaID)
	}
	for _, n := range c.NodeTypes {
		if _, ok := nodeTypes[n]; !ok {
			t.Fatalf("catalog node %q missing from validate set", n)
		}
	}
	if len(c.NodeTypes) != len(nodeTypes) {
		t.Fatalf("node_types len %d want %d", len(c.NodeTypes), len(nodeTypes))
	}
	for _, e := range c.EdgeKinds {
		if _, ok := edgeKinds[e]; !ok {
			t.Fatalf("catalog edge %q missing from validate set", e)
		}
	}
	if len(c.EdgeKinds) != len(edgeKinds) {
		t.Fatalf("edge_kinds len %d want %d", len(c.EdgeKinds), len(edgeKinds))
	}
	if len(c.Tools) != 2 || c.Tools[0] != ToolTransfer || c.Tools[1] != ToolHangup {
		t.Fatalf("tools %+v", c.Tools)
	}
}
