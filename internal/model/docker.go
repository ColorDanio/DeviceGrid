package model

import "time"

type ContainerStatus string

const (
	ContainerRunning ContainerStatus = "running"
	ContainerStopped ContainerStatus = "stopped"
	ContainerPaused  ContainerStatus = "paused"
	ContainerExited  ContainerStatus = "exited"
)

type Container struct {
	ID      string            `json:"id" bson:"_id"`
	NodeID  string            `json:"node_id" bson:"node_id"`
	Name    string            `json:"name" bson:"name"`
	Image   string            `json:"image" bson:"image"`
	Status  ContainerStatus   `json:"status" bson:"status"`
	State   string            `json:"state" bson:"state"`
	Ports   []PortMapping     `json:"ports" bson:"ports"`
	Env     map[string]string `json:"env" bson:"env"`
	Labels  map[string]string `json:"labels" bson:"labels"`
	Created time.Time         `json:"created" bson:"created"`
}

type PortMapping struct {
	HostIP        string `json:"host_ip" bson:"host_ip"`
	HostPort      string `json:"host_port" bson:"host_port"`
	ContainerPort string `json:"container_port" bson:"container_port"`
	Protocol      string `json:"protocol" bson:"protocol"`
}

type ContainerAction string

const (
	ActionStart   ContainerAction = "start"
	ActionStop    ContainerAction = "stop"
	ActionRestart ContainerAction = "restart"
	ActionRemove  ContainerAction = "remove"
	ActionPause   ContainerAction = "pause"
	ActionUnpause ContainerAction = "unpause"
)

type Image struct {
	ID      string    `json:"id" bson:"_id"`
	NodeID  string    `json:"node_id" bson:"node_id"`
	Tags    []string  `json:"tags" bson:"tags"`
	Size    int64     `json:"size" bson:"size"`
	Created time.Time `json:"created" bson:"created"`
}

type ComposeProject struct {
	NodeID    string    `json:"node_id" bson:"node_id"`
	Name      string    `json:"name" bson:"name"`
	Config    string    `json:"config" bson:"config"`
	Status    string    `json:"status" bson:"status"`
	Services  int       `json:"services" bson:"services"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
