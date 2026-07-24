package cloudformation

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// CreateStackParams contains parameters for creating a CloudFormation stack
type CreateStackParams struct {
	StackName    string
	TemplateBody string
	Parameters   map[string]string
	Capabilities []types.Capability
	Tags         []types.Tag
	WaitTimeout  time.Duration
	NoWait       bool // if true, skip waiting for stack creation to complete
}

// UpdateStackParams contains parameters for updating a CloudFormation stack
type UpdateStackParams struct {
	StackName           string
	TemplateBody        string
	UsePreviousTemplate bool // if true, TemplateBody is ignored and the existing template is reused
	Parameters          map[string]string
	Capabilities        []types.Capability
	WaitTimeout         time.Duration
	NoWait              bool // if true, skip waiting for stack update to complete
}

// StackOutput contains the outputs from a CloudFormation stack
type StackOutput struct {
	StackID string
	Outputs map[string]string
}

// StackInfo contains information about a CloudFormation stack
type StackInfo struct {
	StackName    string
	StackID      string
	Status       string
	CreationTime *time.Time
	Outputs      map[string]string
	Tags         map[string]string
}

// StackEvent represents a CloudFormation stack event
type StackEvent struct {
	EventID              string
	StackName            string
	LogicalResourceID    string
	PhysicalResourceID   string
	ResourceType         string
	Timestamp            *time.Time
	ResourceStatus       string
	ResourceStatusReason string
}

// IsComplete checks if a stack is in a complete state
func (s *StackInfo) IsComplete() bool {
	switch s.Status {
	case "CREATE_COMPLETE",
		"UPDATE_COMPLETE",
		"ROLLBACK_COMPLETE",
		"UPDATE_ROLLBACK_COMPLETE",
		"DELETE_COMPLETE":
		return true
	default:
		return false
	}
}

// IsInProgress checks if a stack operation is in progress
func (s *StackInfo) IsInProgress() bool {
	switch s.Status {
	case "CREATE_IN_PROGRESS",
		"UPDATE_IN_PROGRESS",
		"ROLLBACK_IN_PROGRESS",
		"UPDATE_ROLLBACK_IN_PROGRESS",
		"DELETE_IN_PROGRESS":
		return true
	default:
		return false
	}
}

// IsFailed checks if a stack is in a failed state
func (s *StackInfo) IsFailed() bool {
	switch s.Status {
	case "CREATE_FAILED",
		"UPDATE_FAILED",
		"ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_FAILED",
		"DELETE_FAILED":
		return true
	default:
		return false
	}
}
