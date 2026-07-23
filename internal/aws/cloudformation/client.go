package cloudformation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// Client wraps the AWS CloudFormation SDK client
type Client struct {
	cfn *cloudformation.Client
}

// NewClient creates a new CloudFormation client
func NewClient(cfg aws.Config) *Client {
	return &Client{
		cfn: cloudformation.NewFromConfig(cfg),
	}
}

// CreateStack creates a new CloudFormation stack
func (c *Client) CreateStack(ctx context.Context, params *CreateStackParams) (*StackOutput, error) {
	input := &cloudformation.CreateStackInput{
		StackName:    aws.String(params.StackName),
		TemplateBody: aws.String(params.TemplateBody),
		Capabilities: params.Capabilities,
		Tags:         params.Tags,
	}

	if len(params.Parameters) > 0 {
		cfParams := make([]types.Parameter, 0, len(params.Parameters))
		for k, v := range params.Parameters {
			cfParams = append(cfParams, types.Parameter{
				ParameterKey:   aws.String(k),
				ParameterValue: aws.String(v),
			})
		}
		input.Parameters = cfParams
	}

	_, err := c.cfn.CreateStack(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}

	// Wait for stack creation to complete
	waiter := cloudformation.NewStackCreateCompleteWaiter(c.cfn)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(params.StackName),
	}, params.WaitTimeout)
	if err != nil {
		return nil, c.wrapWaiterError(ctx, params.StackName, "creation", err)
	}

	// Get stack outputs
	return c.GetStackOutputs(ctx, params.StackName)
}

// UpdateStack updates an existing CloudFormation stack
func (c *Client) UpdateStack(ctx context.Context, params *UpdateStackParams) (*StackOutput, error) {
	input := &cloudformation.UpdateStackInput{
		StackName:    aws.String(params.StackName),
		Capabilities: params.Capabilities,
	}

	if params.UsePreviousTemplate {
		input.UsePreviousTemplate = aws.Bool(true)
	} else {
		input.TemplateBody = aws.String(params.TemplateBody)
	}

	if len(params.Parameters) > 0 {
		cfParams := make([]types.Parameter, 0, len(params.Parameters))
		for k, v := range params.Parameters {
			cfParams = append(cfParams, types.Parameter{
				ParameterKey:   aws.String(k),
				ParameterValue: aws.String(v),
			})
		}
		input.Parameters = cfParams
	}

	_, err := c.cfn.UpdateStack(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}

	// Wait for stack update to complete
	waiter := cloudformation.NewStackUpdateCompleteWaiter(c.cfn)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(params.StackName),
	}, params.WaitTimeout)
	if err != nil {
		return nil, c.wrapWaiterError(ctx, params.StackName, "update", err)
	}

	// Get stack outputs
	return c.GetStackOutputs(ctx, params.StackName)
}

// DeleteStack deletes a CloudFormation stack
func (c *Client) DeleteStack(ctx context.Context, stackName string, waitTimeout time.Duration) error {
	_, err := c.cfn.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return wrapError(err)
	}

	// Wait for stack deletion to complete
	waiter := cloudformation.NewStackDeleteCompleteWaiter(c.cfn)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waitTimeout)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// DescribeStack gets information about a CloudFormation stack
func (c *Client) DescribeStack(ctx context.Context, stackName string) (*StackInfo, error) {
	result, err := c.cfn.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, wrapError(err)
	}

	if len(result.Stacks) == 0 {
		return nil, &StackNotFoundError{StackName: stackName}
	}

	stack := result.Stacks[0]
	return &StackInfo{
		StackName:    aws.ToString(stack.StackName),
		StackID:      aws.ToString(stack.StackId),
		Status:       string(stack.StackStatus),
		CreationTime: stack.CreationTime,
		Outputs:      convertOutputs(stack.Outputs),
		Tags:         convertTags(stack.Tags),
	}, nil
}

// GetStackOutputs retrieves the outputs from a CloudFormation stack
func (c *Client) GetStackOutputs(ctx context.Context, stackName string) (*StackOutput, error) {
	info, err := c.DescribeStack(ctx, stackName)
	if err != nil {
		return nil, err
	}

	return &StackOutput{
		StackID: info.StackID,
		Outputs: info.Outputs,
	}, nil
}

// ListStacks lists CloudFormation stacks with an optional name prefix filter
func (c *Client) ListStacks(ctx context.Context, prefix string) ([]StackInfo, error) {
	input := &cloudformation.ListStacksInput{
		StackStatusFilter: []types.StackStatus{
			types.StackStatusCreateComplete,
			types.StackStatusUpdateComplete,
			types.StackStatusRollbackComplete,
			types.StackStatusUpdateRollbackComplete,
		},
	}

	var stacks []StackInfo
	paginator := cloudformation.NewListStacksPaginator(c.cfn, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, wrapError(err)
		}

		for _, summary := range page.StackSummaries {
			stackName := aws.ToString(summary.StackName)

			// Filter by prefix if provided
			if prefix != "" && !strings.HasPrefix(stackName, prefix) {
				continue
			}

			stacks = append(stacks, StackInfo{
				StackName:    stackName,
				StackID:      aws.ToString(summary.StackId),
				Status:       string(summary.StackStatus),
				CreationTime: summary.CreationTime,
			})
		}
	}

	return stacks, nil
}

// GetStackEvents retrieves the events for a CloudFormation stack
func (c *Client) GetStackEvents(ctx context.Context, stackName string, limit int) ([]StackEvent, error) {
	input := &cloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	}

	result, err := c.cfn.DescribeStackEvents(ctx, input)
	if err != nil {
		return nil, wrapError(err)
	}

	events := make([]StackEvent, 0, len(result.StackEvents))
	for i, event := range result.StackEvents {
		if limit > 0 && i >= limit {
			break
		}

		events = append(events, StackEvent{
			EventID:              aws.ToString(event.EventId),
			StackName:            aws.ToString(event.StackName),
			LogicalResourceID:    aws.ToString(event.LogicalResourceId),
			PhysicalResourceID:   aws.ToString(event.PhysicalResourceId),
			ResourceType:         aws.ToString(event.ResourceType),
			Timestamp:            event.Timestamp,
			ResourceStatus:       string(event.ResourceStatus),
			ResourceStatusReason: aws.ToString(event.ResourceStatusReason),
		})
	}

	return events, nil
}

func (c *Client) wrapWaiterError(ctx context.Context, stackName, operation string, waiterErr error) error {
	events, err := c.GetStackEvents(ctx, stackName, 50)
	if err != nil {
		return fmt.Errorf("stack %s failed (could not retrieve failure details): %w", operation, waiterErr)
	}

	var reasons []string
	for _, e := range events {
		if strings.HasSuffix(e.ResourceStatus, "_FAILED") && e.ResourceStatusReason != "" {
			reasons = append(reasons, fmt.Sprintf("  %s (%s): %s", e.LogicalResourceID, e.ResourceType, e.ResourceStatusReason))
		}
	}

	if len(reasons) == 0 {
		return fmt.Errorf("stack %s failed: %w", operation, waiterErr)
	}

	return fmt.Errorf("stack %s failed:\n%s", operation, strings.Join(reasons, "\n"))
}

// Helper functions

func convertOutputs(outputs []types.Output) map[string]string {
	result := make(map[string]string)
	for _, output := range outputs {
		key := aws.ToString(output.OutputKey)
		value := aws.ToString(output.OutputValue)
		result[key] = value
	}
	return result
}

func convertTags(tags []types.Tag) map[string]string {
	result := make(map[string]string)
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		result[key] = value
	}
	return result
}
