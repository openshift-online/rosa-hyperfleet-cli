package route53

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// CleanHostedZoneForDeletion removes all non-default records (everything except
// SOA and NS) from a Route53 hosted zone so that CloudFormation can delete it.
//
// This is a temporary workaround for OCPBUGS-74960 where HyperShift's CPO
// fails to clean up DNS records (api.*.hypershift.local, *.apps.*.hypershift.local)
// during HCP deletion.
func CleanHostedZoneForDeletion(ctx context.Context, cfg aws.Config, zoneID string) error {
	client := route53.NewFromConfig(cfg)

	var records []types.ResourceRecordSet
	paginator := route53.NewListResourceRecordSetsPaginator(client, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing records in zone %s: %w", zoneID, err)
		}
		records = append(records, page.ResourceRecordSets...)
	}

	var changes []types.Change
	for _, record := range records {
		if record.Type == types.RRTypeSoa || record.Type == types.RRTypeNs {
			continue
		}
		log.Printf("  will delete %s record %s", record.Type, aws.ToString(record.Name))
		changes = append(changes, types.Change{
			Action:            types.ChangeActionDelete,
			ResourceRecordSet: &record,
		})
	}

	if len(changes) == 0 {
		log.Printf("  no orphaned records found in zone %s", zoneID)
		return nil
	}

	var cleanupErr error
	// Route53 ChangeResourceRecordSets supports up to 1000 changes per batch.
	const batchSize = 1000
	for i := 0; i < len(changes); i += batchSize {
		end := i + batchSize
		if end > len(changes) {
			end = len(changes)
		}
		batch := changes[i:end]

		_, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
			ChangeBatch: &types.ChangeBatch{
				Changes: batch,
			},
		})
		if err != nil {
			log.Printf("  warning: failed to delete %d records from zone %s: %v", len(batch), zoneID, err)
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete records from zone %s: %w", zoneID, err))
		} else {
			log.Printf("  deleted %d records from zone %s", len(batch), zoneID)
		}
	}

	return cleanupErr
}
