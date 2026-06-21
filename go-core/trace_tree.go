package core

// TraceNode is a node in the trace tree. It represents a single span
// and its children, forming a hierarchical view of execution.
type TraceNode struct {
	Step     ExecutionStep `json:"step"`
	Children []*TraceNode  `json:"children,omitempty"`
	Depth    int           `json:"depth"`
}

// TraceTree builds a hierarchical tree from a flat list of ExecutionSteps.
// It organizes spans by ParentID to reconstruct the call graph.
type TraceTree struct {
	Roots []*TraceNode `json:"roots"`
}

// BuildTraceTree constructs a TraceTree from a flat list of ExecutionSteps.
// Steps with no ParentID or whose ParentID does not match any SpanID become roots.
func BuildTraceTree(steps []ExecutionStep) *TraceTree {
	if len(steps) == 0 {
		return &TraceTree{}
	}

	// Build a map of SpanID -> node for O(1) lookup
	nodeMap := make(map[string]*TraceNode, len(steps))
	for i := range steps {
		step := steps[i]
		nodeMap[step.SpanID] = &TraceNode{Step: step, Depth: 0}
	}

	// Second pass: assign children to parents
	roots := make([]*TraceNode, 0)
	for _, node := range nodeMap {
		if node.Step.ParentID == "" {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodeMap[node.Step.ParentID]
		if !ok {
			// Parent not found, treat as root
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	// Set depths
	var setDepth func(n *TraceNode, d int)
	setDepth = func(n *TraceNode, d int) {
		n.Depth = d
		for _, child := range n.Children {
			setDepth(child, d+1)
		}
	}
	for _, root := range roots {
		setDepth(root, 0)
	}

	return &TraceTree{Roots: roots}
}

// Flatten returns all nodes in depth-first order.
func (t *TraceTree) Flatten() []*TraceNode {
	result := make([]*TraceNode, 0)
	var dfs func(n *TraceNode)
	dfs = func(n *TraceNode) {
		result = append(result, n)
		for _, child := range n.Children {
			dfs(child)
		}
	}
	for _, root := range t.Roots {
		dfs(root)
	}
	return result
}

// TotalNodes returns the total number of nodes in the tree.
func (t *TraceTree) TotalNodes() int {
	count := 0
	var dfs func(n *TraceNode)
	dfs = func(n *TraceNode) {
		count++
		for _, child := range n.Children {
			dfs(child)
		}
	}
	for _, root := range t.Roots {
		dfs(root)
	}
	return count
}
