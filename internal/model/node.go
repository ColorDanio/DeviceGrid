package model

import "time"

type NodeStatus string

const (
	NodeStatusOnline    NodeStatus = "online"
	NodeStatusOffline   NodeStatus = "offline"
	NodeStatusUntrusted NodeStatus = "untrusted"
	NodeStatusError     NodeStatus = "error"
)

type TransportMode string

const (
	TransportSSH   TransportMode = "ssh"
	TransportAgent TransportMode = "agent"
)

type AuthMode string

const (
	AuthPassword AuthMode = "password"
	AuthKey      AuthMode = "key"
)

type Node struct {
	ID            string      `json:"id" bson:"_id"`
	Name          string      `json:"name" bson:"name"`
	Host          string      `json:"host" bson:"host"`
	Port          int         `json:"port" bson:"port"`
	Username      string      `json:"username" bson:"username"`
	AuthMode      AuthMode    `json:"auth_mode" bson:"auth_mode"`
	PasswordEnc   string      `json:"-" bson:"password_enc"`
	PrivateKeyEnc string      `json:"-" bson:"private_key_enc"`
	TransportMode TransportMode `json:"transport_mode" bson:"transport_mode"`
	AgentPort     int         `json:"agent_port" bson:"agent_port"`
	Status        NodeStatus  `json:"status" bson:"status"`
	Tags          []string    `json:"tags" bson:"tags"`
	OS            string      `json:"os" bson:"os"`
	Arch          string      `json:"arch" bson:"arch"`
	DockerVersion string      `json:"docker_version" bson:"docker_version"`
	RKE2Role      string      `json:"rke2_role" bson:"rke2_role"`
	ClusterID     string      `json:"cluster_id,omitempty" bson:"cluster_id,omitempty"`
	Country       string      `json:"country,omitempty" bson:"country,omitempty"`
	CountryCode   string      `json:"country_code,omitempty" bson:"country_code,omitempty"`
	Region        string      `json:"region,omitempty" bson:"region,omitempty"`
	ISP           string      `json:"isp,omitempty" bson:"isp,omitempty"`
	LastSeenAt    time.Time   `json:"last_seen_at" bson:"last_seen_at"`
	CreatedAt     time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" bson:"updated_at"`
}

type NodeFilter struct {
	Search string
	Status NodeStatus
	Tag    string
}

type NodeMetrics struct {
	NodeID   string  `json:"node_id"`
	CPUUsage float64 `json:"cpu_usage"`
	MemTotal uint64  `json:"mem_total"`
	MemUsed  uint64  `json:"mem_used"`
	DiskTotal uint64 `json:"disk_total"`
	DiskUsed  uint64 `json:"disk_used"`
	LoadAvg1  float64 `json:"load_avg_1"`
	Uptime    uint64  `json:"uptime"`
}

type NodeFacts struct {
	OS            string `json:"os"`
	OSVersion     string `json:"os_version"`
	Arch          string `json:"arch"`
	Kernel        string `json:"kernel"`
	DockerVersion string `json:"docker_version"`
	RKE2Version   string `json:"rke2_version"`
}
