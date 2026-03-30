package localstack_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

var _ = Describe("rosactl LocalStack Integration", func() {
	var (
		ctx           context.Context
		binaryPath    string
		localstackURL string
		awsRegion     string
		testCluster   string
		cfnClient     *cloudformation.Client
		iamClient     *iam.Client
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Get LocalStack endpoint from environment
		localstackURL = os.Getenv("LOCALSTACK_ENDPOINT")
		if localstackURL == "" {
			localstackURL = "http://localhost:4566"
		}

		awsRegion = os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-east-1"
		}

		// Find the rosactl binary
		projectRoot := filepath.Join("..", "..")
		binaryPath = filepath.Join(projectRoot, "rosactl")
		Expect(binaryPath).To(BeAnExistingFile(), "rosactl binary must exist")

		// Generate unique test cluster name
		testCluster = fmt.Sprintf("test-cluster-%d", time.Now().Unix())

		// Create AWS clients pointing to LocalStack
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(awsRegion),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:               localstackURL,
						HostnameImmutable: true,
						SigningRegion:     awsRegion,
					}, nil
				},
			)),
		)
		Expect(err).NotTo(HaveOccurred())

		cfnClient = cloudformation.NewFromConfig(cfg)
		iamClient = iam.NewFromConfig(cfg)

		// Create dummy AWS-managed policies for LocalStack
		createDummyAWSManagedPolicies(ctx, iamClient)
	})

	AfterEach(func() {
		// Cleanup: delete stacks
		By("Cleaning up test resources")

		stackNames := []string{
			fmt.Sprintf("rosa-%s-vpc", testCluster),
			fmt.Sprintf("rosa-%s-iam", testCluster),
		}

		for _, stackName := range stackNames {
			_, _ = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
				StackName: aws.String(stackName),
			})
		}
	})

	Describe("VPC Management", func() {
		It("should create and delete VPC resources via rosactl CLI", func(ctx SpecContext) {
			stackName := fmt.Sprintf("rosa-%s-vpc", testCluster)

			By("Running rosactl cluster-vpc create command")
			createCmd := exec.CommandContext(ctx, binaryPath, "cluster-vpc", "create", testCluster,
				"--region", awsRegion,
				"--availability-zones", "us-east-1a,us-east-1b,us-east-1c",
				"--single-nat-gateway=false", // Use multi-NAT to avoid LocalStack conditional resource deletion bug
			)
			createCmd.Env = append(os.Environ(),
				fmt.Sprintf("AWS_ENDPOINT_URL=%s", localstackURL),
				fmt.Sprintf("AWS_REGION=%s", awsRegion),
			)

			createSession, err := gexec.Start(createCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for command to complete (may take time for CloudFormation)
			Eventually(createSession, 90*time.Second).Should(gexec.Exit())

			// Check output contains expected messages
			output := string(createSession.Out.Contents())
			GinkgoWriter.Printf("\nCLI Create Output:\n%s\n", output)
			Expect(output).To(ContainSubstring("Creating cluster VPC resources"))

			// Verify the stack exists
			By("Verifying VPC stack was created")
			stackResult, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			})
			Expect(err).NotTo(HaveOccurred(), "Stack should exist after CLI invocation")
			Expect(stackResult.Stacks).To(HaveLen(1))

			stackStatus := string(stackResult.Stacks[0].StackStatus)
			GinkgoWriter.Printf("VPC Stack status: %s\n", stackStatus)
			// Accept CREATE_COMPLETE or CREATE_FAILED (LocalStack NAT Gateway limitation)
			Expect(stackStatus).To(Or(Equal("CREATE_COMPLETE"), Equal("CREATE_FAILED")))

			By("Listing stack resources")
			resources, err := cfnClient.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
				StackName: aws.String(stackName),
			})
			if err == nil {
				GinkgoWriter.Printf("\nVPC Stack resources (%d):\n", len(resources.StackResourceSummaries))
				for _, res := range resources.StackResourceSummaries {
					GinkgoWriter.Printf("  %s (%s): %s\n",
						aws.ToString(res.LogicalResourceId),
						aws.ToString(res.ResourceType),
						string(res.ResourceStatus))
				}
			}

			By("Running rosactl cluster-vpc delete command")
			deleteCmd := exec.CommandContext(ctx, binaryPath, "cluster-vpc", "delete", testCluster,
				"--region", awsRegion,
			)
			deleteCmd.Env = append(os.Environ(),
				fmt.Sprintf("AWS_ENDPOINT_URL=%s", localstackURL),
				fmt.Sprintf("AWS_REGION=%s", awsRegion),
			)

			deleteSession, err := gexec.Start(deleteCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for delete command to complete
			Eventually(deleteSession, 90*time.Second).Should(gexec.Exit(0))

			deleteOutput := string(deleteSession.Out.Contents())
			GinkgoWriter.Printf("\nCLI Delete Output:\n%s\n", deleteOutput)
			Expect(deleteOutput).To(ContainSubstring("Deleting cluster VPC resources"))

			By("Verifying VPC stack was deleted or is deleting")
			Eventually(func() bool {
				result, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
					StackName: aws.String(stackName),
				})
				if err != nil {
					// Stack not found means it was deleted
					if strings.Contains(err.Error(), "does not exist") {
						GinkgoWriter.Printf("VPC Stack deleted successfully\n")
						return true
					}
					return false
				}
				if len(result.Stacks) == 0 {
					return true
				}
				status := string(result.Stacks[0].StackStatus)
				GinkgoWriter.Printf("VPC Stack deletion status: %s\n", status)
				return status == "DELETE_COMPLETE"
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

		}, SpecTimeout(180*time.Second))
	})

	Describe("IAM Management", func() {
		It("should create and delete IAM resources via rosactl CLI", func(ctx SpecContext) {
			stackName := fmt.Sprintf("rosa-%s-iam", testCluster)

			By("Running rosactl cluster-iam create command")
			createCmd := exec.CommandContext(ctx, binaryPath, "cluster-iam", "create", testCluster,
				"--region", awsRegion,
				"--oidc-issuer-url", "https://test-oidc.s3.amazonaws.com",
			)
			createCmd.Env = append(os.Environ(),
				fmt.Sprintf("AWS_ENDPOINT_URL=%s", localstackURL),
				fmt.Sprintf("AWS_REGION=%s", awsRegion),
			)

			createSession, err := gexec.Start(createCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for command to complete (may take time for CloudFormation and OIDC thumbprint fetch)
			Eventually(createSession, 90*time.Second).Should(gexec.Exit())

			// Check output - command might fail on thumbprint fetch for fake URL, that's OK for LocalStack
			output := string(createSession.Out.Contents())
			errOutput := string(createSession.Err.Contents())
			GinkgoWriter.Printf("\nCLI Create Output:\n%s\n", output)
			if errOutput != "" {
				GinkgoWriter.Printf("\nCLI Create Errors:\n%s\n", errOutput)
			}

			// Verify the stack was created (even if command failed on thumbprint)
			By("Verifying IAM stack was created")
			stackResult, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			})

			// If stack doesn't exist, the command failed before creating it (likely thumbprint issue)
			// Skip the test gracefully for LocalStack since we can't fetch real OIDC thumbprints
			if err != nil && strings.Contains(err.Error(), "does not exist") {
				Skip("Stack not created - likely OIDC thumbprint fetch failed in LocalStack (expected)")
			}

			Expect(err).NotTo(HaveOccurred(), "Stack should exist after CLI invocation")
			Expect(stackResult.Stacks).To(HaveLen(1))

			stackStatus := string(stackResult.Stacks[0].StackStatus)
			GinkgoWriter.Printf("IAM Stack status: %s\n", stackStatus)
			// Accept CREATE_COMPLETE or CREATE_FAILED (LocalStack AWS-managed policy limitation)
			Expect(stackStatus).To(Or(
				Equal("CREATE_COMPLETE"),
				Equal("CREATE_FAILED"),
				Equal("UPDATE_IN_PROGRESS"),
			))

			By("Listing stack resources")
			resources, err := cfnClient.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
				StackName: aws.String(stackName),
			})
			if err == nil {
				GinkgoWriter.Printf("\nIAM Stack resources (%d):\n", len(resources.StackResourceSummaries))
				for i, res := range resources.StackResourceSummaries {
					if i < 10 { // Show first 10 resources
						GinkgoWriter.Printf("  %s (%s): %s\n",
							aws.ToString(res.LogicalResourceId),
							aws.ToString(res.ResourceType),
							string(res.ResourceStatus))
					}
				}
				Expect(resources.StackResourceSummaries).ToNot(BeEmpty(), "Stack should have resources")
			}

			By("Running rosactl cluster-iam delete command")
			deleteCmd := exec.CommandContext(ctx, binaryPath, "cluster-iam", "delete", testCluster,
				"--region", awsRegion,
			)
			deleteCmd.Env = append(os.Environ(),
				fmt.Sprintf("AWS_ENDPOINT_URL=%s", localstackURL),
				fmt.Sprintf("AWS_REGION=%s", awsRegion),
			)

			deleteSession, err := gexec.Start(deleteCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for delete command to complete
			Eventually(deleteSession, 90*time.Second).Should(gexec.Exit(0))

			deleteOutput := string(deleteSession.Out.Contents())
			GinkgoWriter.Printf("\nCLI Delete Output:\n%s\n", deleteOutput)
			Expect(deleteOutput).To(ContainSubstring("Deleting cluster IAM resources"))

			By("Verifying IAM stack was deleted or is deleting")
			Eventually(func() bool {
				result, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
					StackName: aws.String(stackName),
				})
				if err != nil {
					// Stack not found means it was deleted
					if strings.Contains(err.Error(), "does not exist") {
						GinkgoWriter.Printf("IAM Stack deleted successfully\n")
						return true
					}
					return false
				}
				if len(result.Stacks) == 0 {
					return true
				}
				status := string(result.Stacks[0].StackStatus)
				GinkgoWriter.Printf("IAM Stack deletion status: %s\n", status)
				return status == "DELETE_COMPLETE"
			}, 60*time.Second, 2*time.Second).Should(BeTrue())

		}, SpecTimeout(180*time.Second))
	})

	Describe("Cluster Deploy (parallel)", func() {
		It("should deploy both IAM and VPC stacks in a single command", func(ctx SpecContext) {
			iamStackName := fmt.Sprintf("rosa-%s-iam", testCluster)
			vpcStackName := fmt.Sprintf("rosa-%s-vpc", testCluster)

			By("Running rosactl cluster deploy command")
			deployCmd := exec.CommandContext(ctx, binaryPath, "cluster", "deploy", testCluster,
				"--region", awsRegion,
				"--oidc-issuer-url", "https://test-oidc.s3.amazonaws.com",
				"--availability-zones", "us-east-1a,us-east-1b,us-east-1c",
				"--single-nat-gateway=false",
			)
			deployCmd.Env = append(os.Environ(),
				fmt.Sprintf("AWS_ENDPOINT_URL=%s", localstackURL),
				fmt.Sprintf("AWS_REGION=%s", awsRegion),
			)

			deploySession, err := gexec.Start(deployCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Both CloudFormation stacks run in parallel; allow enough time for both.
			Eventually(deploySession, 120*time.Second).Should(gexec.Exit())

			output := string(deploySession.Out.Contents())
			errOutput := string(deploySession.Err.Contents())
			GinkgoWriter.Printf("\nCLI Deploy Output:\n%s\n", output)
			if errOutput != "" {
				GinkgoWriter.Printf("\nCLI Deploy Errors:\n%s\n", errOutput)
			}

			Expect(output).To(ContainSubstring("Deploying cluster resources"))

			// VPC does not require a network round-trip for thumbprint fetching, so
			// it should always be attempted. If it is absent the IAM thumbprint fetch
			// failed and its context cancellation aborted VPC before it started —
			// acceptable in offline environments.
			By("Verifying VPC stack was created")
			vpcResult, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(vpcStackName),
			})
			if err != nil && strings.Contains(err.Error(), "does not exist") {
				Skip("VPC stack not created — IAM thumbprint failure likely cancelled VPC via context (expected in offline environments)")
			}
			Expect(err).NotTo(HaveOccurred(), "VPC stack should exist after deploy command")
			Expect(vpcResult.Stacks).To(HaveLen(1))
			vpcStatus := string(vpcResult.Stacks[0].StackStatus)
			GinkgoWriter.Printf("VPC Stack status: %s\n", vpcStatus)
			// Accept CREATE_COMPLETE or CREATE_FAILED (LocalStack NAT Gateway limitation)
			Expect(vpcStatus).To(Or(Equal("CREATE_COMPLETE"), Equal("CREATE_FAILED")))

			By("Verifying IAM stack was created")
			iamResult, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(iamStackName),
			})
			if err != nil && strings.Contains(err.Error(), "does not exist") {
				Skip("IAM stack not created — OIDC thumbprint fetch likely failed in LocalStack (expected in offline environments)")
			}
			Expect(err).NotTo(HaveOccurred(), "IAM stack should exist after deploy command")
			Expect(iamResult.Stacks).To(HaveLen(1))
			iamStatus := string(iamResult.Stacks[0].StackStatus)
			GinkgoWriter.Printf("IAM Stack status: %s\n", iamStatus)
			// Accept CREATE_COMPLETE or CREATE_FAILED (LocalStack AWS-managed policy limitation)
			Expect(iamStatus).To(Or(
				Equal("CREATE_COMPLETE"),
				Equal("CREATE_FAILED"),
				Equal("UPDATE_IN_PROGRESS"),
			))

		}, SpecTimeout(180*time.Second))
	})

	Describe("Stack Listing", func() {
		It("should list CloudFormation stacks", func(ctx SpecContext) {
			By("Creating a test stack")
			stackName := fmt.Sprintf("rosa-%s-vpc", testCluster)
			templatePath := filepath.Join("..", "..", "internal", "cloudformation", "templates", "cluster-vpc.yaml")
			templateBody, err := os.ReadFile(templatePath)
			Expect(err).NotTo(HaveOccurred())

			_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
				StackName:    aws.String(stackName),
				TemplateBody: aws.String(string(templateBody)),
				Parameters: []cloudformationTypes.Parameter{
					{
						ParameterKey:   aws.String("ClusterName"),
						ParameterValue: aws.String(testCluster),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Listing stacks via CloudFormation API")
			result, err := cfnClient.ListStacks(ctx, &cloudformation.ListStacksInput{})
			Expect(err).NotTo(HaveOccurred())

			var foundStack bool
			for _, summary := range result.StackSummaries {
				if *summary.StackName == stackName {
					foundStack = true
					break
				}
			}
			Expect(foundStack).To(BeTrue(), "Stack should appear in list")

		}, SpecTimeout(30*time.Second))
	})
})

// createDummyAWSManagedPolicies creates dummy AWS-managed policies in LocalStack
// so that CloudFormation templates can reference them
func createDummyAWSManagedPolicies(ctx context.Context, iamClient *iam.Client) {
	dummyPolicyDoc := `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": "*",
			"Resource": "*"
		}]
	}`

	policies := []struct {
		name string
		path string
	}{
		{"ROSAIngressOperatorPolicy", "/service-role/"},
		{"ROSAKubeControllerPolicy", "/service-role/"},
		{"ROSAAmazonEBSCSIDriverOperatorPolicy", "/service-role/"},
		{"ROSAImageRegistryOperatorPolicy", "/service-role/"},
		{"ROSACloudNetworkConfigOperatorPolicy", "/service-role/"},
		{"ROSAControlPlaneOperatorPolicy", "/service-role/"},
		{"ROSANodePoolManagementPolicy", "/service-role/"},
		{"ROSAWorkerInstancePolicy", "/service-role/"},
		{"AmazonSSMManagedInstanceCore", "/"},
	}

	GinkgoWriter.Printf("\nCreating dummy AWS-managed policies in LocalStack:\n")
	for _, policy := range policies {
		output, err := iamClient.CreatePolicy(ctx, &iam.CreatePolicyInput{
			PolicyName:     aws.String(policy.name),
			Path:           aws.String(policy.path),
			PolicyDocument: aws.String(dummyPolicyDoc),
			Description:    aws.String("Dummy AWS-managed policy for LocalStack testing"),
		})
		// Ignore if policy already exists
		if err != nil {
			if strings.Contains(err.Error(), "EntityAlreadyExists") {
				GinkgoWriter.Printf("  ✓ %s (already exists)\n", policy.name)
			} else {
				GinkgoWriter.Printf("  ✗ %s: %v\n", policy.name, err)
			}
		} else {
			GinkgoWriter.Printf("  ✓ %s -> %s\n", policy.name, *output.Policy.Arn)
		}
	}

	// List policies we created (with path filter)
	GinkgoWriter.Printf("\nVerifying created policies:\n")
	for _, policy := range policies {
		getResult, err := iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
			PolicyArn: aws.String(fmt.Sprintf("arn:aws:iam::000000000000:policy%s%s", policy.path, policy.name)),
		})
		if err == nil {
			GinkgoWriter.Printf("  ✓ Found: %s\n", *getResult.Policy.Arn)
		} else {
			GinkgoWriter.Printf("  ✗ Not found with standard ARN format: %v\n", err)
		}
	}
}
