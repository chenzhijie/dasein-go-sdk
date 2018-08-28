// Package trickle allows to build trickle DAGs.
// In this type of DAG, non-leave nodes are first filled
// with data leaves, and then incorporate "layers" of subtrees
// as additional links.
//
// Each layer is a trickle sub-tree and is limited by an increasing
// maximum depth. Thus, the nodes first layer
// can only hold leaves (depth 1) but subsequent layers can grow deeper.
// By default, this module places 4 nodes per layer (that is, 4 subtrees
// of the same maximum depth before increasing it).
//
// Trickle DAGs are very good for sequentially reading data, as the
// first data leaves are directly reachable from the root and those
// coming next are always nearby. They are
// suited for things like streaming applications.
package trickle

import (
	h "github.com/daseinio/dasein-go-sdk/importer/helpers"

	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// layerRepeat specifies how many times to append a child tree of a
// given depth. Higher values increase the width of a given node, which
// improves seek speeds.
const layerRepeat = 4

// Layout builds a new DAG with the trickle format using the provided
// DagBuilderHelper. See the module's description for a more detailed
// explanation.
func Layout(db *h.DagBuilderHelper) (ipld.Node, []*h.UnixfsNode, error) {
	root := db.NewUnixfsNode()
	list, err := fillTrickleRec(db, root, -1)
	if err != nil {
		return nil, nil, err
	}

	out, err := db.Add(root)
	if err != nil {
		return nil, nil, err
	}

	return out, list, nil
}

// fillTrickleRec creates a trickle (sub-)tree with an optional maximum specified depth
// in the case maxDepth is greater than zero, or with unlimited depth otherwise
// (where the DAG builder will signal the end of data to end the function).
func fillTrickleRec(db *h.DagBuilderHelper, node *h.UnixfsNode, maxDepth int) ([]*h.UnixfsNode, error) {
	// Always do this, even in the base case
	list := make([]*h.UnixfsNode, 0)
	for node.NumChildren() < db.Maxlinks() && !db.Done() {
		child, err := db.GetNextDataNode()
		if err != nil {
			return nil, err
		}

		if err := node.AddChild(child, db); err != nil {
			return nil, err
		}
		list = append(list, child)
	}

	for depth := 1; ; depth++ {
		// Apply depth limit only if the parameter is set (> 0).
		if maxDepth > 0 && depth == maxDepth {
			return list, nil
		}
		for layer := 0; layer < layerRepeat; layer++ {
			if db.Done() {
				return list, nil
			}

			nextChild := db.NewUnixfsNode()
			l, err := fillTrickleRec(db, nextChild, depth)
			if err != nil {
				return nil, err
			}
			if l != nil && len(l) > 0 {
				list = append(list, l...)
			}
			if err := node.AddChild(nextChild, db); err != nil {
				return nil, err
			}
			list = append(list, nextChild)
		}
	}

}
