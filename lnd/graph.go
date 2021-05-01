package lnd

import (
	"fmt"

	"github.com/lightningnetwork/lnd/lnrpc"
)

func AllNodeChannels(graph *lnrpc.ChannelGraph,
	nodePubKey string) []*lnrpc.ChannelEdge {
	
	var result []*lnrpc.ChannelEdge
	for _, edge := range graph.Edges {
		if edge.Node1Pub != nodePubKey && edge.Node2Pub != nodePubKey {
			continue
		}
		
		result = append(result, edge)
	}
	
	return result
}

func FindNode(graph *lnrpc.ChannelGraph,
	nodePubKey string) (*lnrpc.LightningNode, error) {
	
	for _, node := range graph.Nodes {
		if node.PubKey == nodePubKey {
			return node, nil
		}
	}
	
	return nil, fmt.Errorf("node %s not found in graph", nodePubKey)
}