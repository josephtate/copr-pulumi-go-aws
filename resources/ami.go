package resources

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// GetLatestRocky9Ami fetches the latest Rocky 9 AMI ID from AWS
func GetLatestRocky9Ami(ctx *pulumi.Context) (string, error) {
	ami, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
		MostRecent: pulumi.BoolRef(true),
		Owners:     []string{"792107900819"},
		Filters: []ec2.GetAmiFilter{
			{
				Name: "name",
				Values: []string{
					"Rocky-9-EC2-LVM-9.*",
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
