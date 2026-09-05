package flow

// NodeTypes returns the closed V1 node type list (docs/03 / coral.flow.v1).
func NodeTypes() []string {
	return []string{
		NodeEntry, NodeSpeak, NodeListenChoice, NodeListenLanguage,
		NodeListenSlot, NodeDecide, NodeInform, NodeTool, NodeEnd,
	}
}

// EdgeKinds returns the closed V1 edge kind list.
func EdgeKinds() []string {
	return []string{
		EdgeNext, EdgeOption, EdgeIntent, EdgeRetry, EdgeBack,
		EdgeSkip, EdgeRepair, EdgeToolResult, EdgeGlobal,
	}
}

// Tools returns Tool node verbs.
func Tools() []string {
	return []string{ToolTransfer, ToolHangup}
}

// MatrixActions returns allowed matrix row actions (P2.9).
func MatrixActions() []string {
	return []string{ToolTransfer}
}

// GraphCatalog is the flow-related subset of GET /v1/meta/catalog (U.1).
type GraphCatalog struct {
	SchemaID      string   `json:"schema_id"`
	NodeTypes     []string `json:"node_types"`
	EdgeKinds     []string `json:"edge_kinds"`
	Tools         []string `json:"tools"`
	MatrixActions []string `json:"matrix_actions"`
}

// Catalog returns graph enums aligned with Validate.
func Catalog() GraphCatalog {
	return GraphCatalog{
		SchemaID:      SchemaID,
		NodeTypes:     NodeTypes(),
		EdgeKinds:     EdgeKinds(),
		Tools:         Tools(),
		MatrixActions: MatrixActions(),
	}
}
