package model

import "time"

type DeployTaskType string

const (
	DeployScript  DeployTaskType = "script"
	DeployFile    DeployTaskType = "file"
	DeployPackage DeployTaskType = "package"
)

type DeployTaskStatus string

const (
	DeployPending   DeployTaskStatus = "pending"
	DeployRunning   DeployTaskStatus = "running"
	DeployCompleted DeployTaskStatus = "completed"
	DeployFailed    DeployTaskStatus = "failed"
	DeployCancelled DeployTaskStatus = "cancelled"
)

type DeployTask struct {
	ID          string           `json:"id" bson:"_id"`
	Name        string           `json:"name" bson:"name"`
	Type        DeployTaskType   `json:"type" bson:"type"`
	NodeIDs     []string         `json:"node_ids" bson:"node_ids"`
	Payload     string           `json:"payload" bson:"payload"`
	Timeout     int              `json:"timeout" bson:"timeout"`
	Concurrency int              `json:"concurrency" bson:"concurrency"`
	Status      DeployTaskStatus `json:"status" bson:"status"`
	CreatedBy   string           `json:"created_by" bson:"created_by"`
	CreatedAt   time.Time        `json:"created_at" bson:"created_at"`
	StartedAt   *time.Time       `json:"started_at,omitempty" bson:"started_at,omitempty"`
	FinishedAt  *time.Time       `json:"finished_at,omitempty" bson:"finished_at,omitempty"`
}

type DeployResultStatus string

const (
	ResultSuccess DeployResultStatus = "success"
	ResultFailed  DeployResultStatus = "failed"
	ResultTimeout DeployResultStatus = "timeout"
	ResultRunning DeployResultStatus = "running"
)

type DeployResult struct {
	ID         string             `json:"id" bson:"_id"`
	TaskID     string             `json:"task_id" bson:"task_id"`
	NodeID     string             `json:"node_id" bson:"node_id"`
	NodeName   string             `json:"node_name" bson:"node_name"`
	Status     DeployResultStatus `json:"status" bson:"status"`
	ExitCode   int                `json:"exit_code" bson:"exit_code"`
	Output     string             `json:"output" bson:"output"`
	Error      string             `json:"error" bson:"error"`
	Duration   int64              `json:"duration_ms" bson:"duration_ms"`
	StartedAt  time.Time          `json:"started_at" bson:"started_at"`
	FinishedAt *time.Time         `json:"finished_at,omitempty" bson:"finished_at,omitempty"`
}
