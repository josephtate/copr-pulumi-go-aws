package resources

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// GetLatestRocky9Ami fetches the latest Rocky 9 AMI ID from AWS
func GetLatestFedoraAmi(ctx *pulumi.Context, fVersion int) (string, error) {
	ami, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
		MostRecent: pulumi.BoolRef(true),
		Owners:     []string{"125523088429"},
		Filters: []ec2.GetAmiFilter{
			{
				Name: "name",
				Values: []string{
					fmt.Sprintf("Fedora-Cloud-Base-AmazonEC2.*-%d-*hvm-*-gp3-*", fVersion),
				},
			},
			{
				Name: "architecture",
				Values: []string{
					"x86_64",
				},
			},
		},
	}, nil)
	if err != nil {
		return "", err
	}
	return ami.Id, nil
}
