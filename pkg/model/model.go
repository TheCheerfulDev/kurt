package model

import "time"

type ResourceKind string

const (
	KindDeployment          ResourceKind = "Deployment"
	KindStatefulSet         ResourceKind = "StatefulSet"
	KindReplicaSet          ResourceKind = "ReplicaSet"
	KindPod                 ResourceKind = "Pod"
	KindService             ResourceKind = "Service"
	KindVirtualService      ResourceKind = "VirtualService"
	KindGateway             ResourceKind = "Gateway"
	KindIngress             ResourceKind = "Ingress"
	KindAuthorizationPolicy ResourceKind = "AuthorizationPolicy"
	KindDestinationRule     ResourceKind = "DestinationRule"
)

type Node struct {
	Kind      ResourceKind
	Name      string
	Namespace string
	Ready     string
	Reason    string
	Status    string
	Hosts     string
	CreatedAt time.Time
	Children  []*Node
}

func NewNode(kind ResourceKind, name, namespace string) *Node {
	return &Node{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Ready:     "-",
		Status:    "-",
		Children:  make([]*Node, 0),
	}
}

func (n *Node) AddChild(child *Node) {
	n.Children = append(n.Children, child)
}

// FilterTree returns a copy of the tree with nodes of excluded kinds removed.
// Children of excluded nodes are promoted to the excluded node's parent.
// The root node is never excluded (even if its kind is in the set).
func FilterTree(root *Node, exclude map[ResourceKind]bool) *Node {
	if len(exclude) == 0 {
		return root
	}
	copy := *root
	copy.Children = filterChildren(root.Children, exclude)
	return &copy
}

func filterChildren(children []*Node, exclude map[ResourceKind]bool) []*Node {
	var result []*Node
	for _, child := range children {
		if exclude[child.Kind] {
			// Skip this node but promote its children.
			result = append(result, filterChildren(child.Children, exclude)...)
		} else {
			copy := *child
			copy.Children = filterChildren(child.Children, exclude)
			result = append(result, &copy)
		}
	}
	return result
}
