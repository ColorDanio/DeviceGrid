package model

import "time"

type ClusterStatus string

const (
	ClusterHealthy      ClusterStatus = "healthy"
	ClusterDegraded     ClusterStatus = "degraded"
	ClusterProvisioning ClusterStatus = "provisioning"
	ClusterError        ClusterStatus = "error"
)

type ClusterNodeRole string

const (
	RoleServer ClusterNodeRole = "server"
	RoleAgent  ClusterNodeRole = "agent"
)

type Cluster struct {
	ID         string        `json:"id" bson:"_id"`
	Name       string        `json:"name" bson:"name"`
	Version    string        `json:"version" bson:"version"`
	ServerNode string        `json:"server_node" bson:"server_node"`
	Config     string        `json:"config" bson:"config"`
	Nodes      []ClusterNode `json:"nodes" bson:"nodes"`
	Status     ClusterStatus `json:"status" bson:"status"`
	CreatedAt  time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at" bson:"updated_at"`
}

type ClusterNode struct {
	NodeID string           `json:"node_id" bson:"node_id"`
	Role   ClusterNodeRole  `json:"role" bson:"role"`
	Ready  bool             `json:"ready" bson:"ready"`
}
