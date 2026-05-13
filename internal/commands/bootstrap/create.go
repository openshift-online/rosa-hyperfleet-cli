package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/aws/cloudformation"
	"github.com/openshift-online/rosa-regional-platform-cli/internal/cloudformation/templates"
	"github.com/spf13/cobra"
)

const (
	defaultStackName    = "rosa-regional-platform-lambda"
	defaultFunctionName = "rosa-regional-platform-lambda"
	defaultTimeout      = 10 * time.Minute
)

type createOptions struct {
	imageURI     string
	functionName string
	region       string
	stackName    string
	noWait       bool
}

func newCreateCommand() *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the Lambda function bootstrap stack",
		Long: `Create the Lambda function infrastructure via CloudFormation.

This command deploys a Lambda function using a container image from ECR.
The Lambda function will be used to apply cluster IAM CloudFormation templates.

Example:
  rosactl bootstrap create \
    --image-uri 123456789012.dkr.ecr.us-east-1.amazonaws.com/rosa-cli:latest \
    --region us-east-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.imageURI, "image-uri", "", "Container image URI from ECR (required)")
	cmd.Flags().StringVar(&opts.functionName, "function-name", defaultFunctionName, "Name of the Lambda function")
	cmd.Flags().StringVar(&opts.region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&opts.stackName, "stack-name", defaultStackName, "Name of the CloudFormation stack")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "Return immediately without waiting for stack creation to complete")

	_ = cmd.MarkFlagRequired("image-uri")
	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func runCreate(ctx context.Context, opts *createOptions) error {
	fmt.Println("🚀 Creating Lambda bootstrap stack...")
	fmt.Printf("   Stack name: %s\n", opts.stackName)
	fmt.Printf("   Function name: %s\n", opts.functionName)
	fmt.Printf("   Container image: %s\n", opts.imageURI)
	fmt.Printf("   Region: %s\n", opts.region)
	fmt.Println()

	// Read template file from embedded templates
	templateBody, err := templates.Read("lambda-bootstrap.yaml")
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create CloudFormation client
	cfnClient := cloudformation.NewClient(cfg)

	// Prepare stack parameters
	params := &cloudformation.CreateStackParams{
		StackName:    opts.stackName,
		TemplateBody: templateBody,
		Parameters: map[string]string{
			"ContainerImageURI": opts.imageURI,
			"FunctionName":      opts.functionName,
		},
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
		},
		Tags: []types.Tag{
			{
				Key:   stringPtr("ManagedBy"),
				Value: stringPtr("rosactl"),
			},
			{
				Key:   stringPtr("Component"),
				Value: stringPtr("lambda-bootstrap"),
			},
		},
		WaitTimeout: defaultTimeout,
		NoWait:      opts.noWait,
	}

	fmt.Println("📋 Creating CloudFormation stack...")

	// Create stack
	output, err := cfnClient.CreateStack(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	if opts.noWait {
		fmt.Println("✅ Stack creation submitted!")
		fmt.Printf("   Stack ID: %s\n", output.StackID)
	} else {
		fmt.Println("✅ Stack created successfully!")
		fmt.Println()
		fmt.Println("Outputs:")
		for key, value := range output.Outputs {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	return nil
}

func stringPtr(s string) *string {
	return &s
}
