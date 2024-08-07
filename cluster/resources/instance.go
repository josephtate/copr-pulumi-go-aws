package resources

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Please note that "backend" is named such because it is the executing engine of COPR, not the due to the typical
// frontend/backend development architecture. It is not the backend of the application.
func CreateInstance(
	ctx *pulumi.Context,
	config *config.Config,
	name string,
	securityGroups map[string]*ec2.SecurityGroup,
) (*ec2.Instance, error) {
	resourcePrefix := config.Require("resourcePrefix")
	instanceType := config.Require("instanceTypeBackend")
	subnetID := getFirstSubnetID(config, true)

	amiID, err := GetLatestRocky9Ami(ctx)
	if err != nil {
		return nil, err
	}
	// Launch an EC2 instance with the resourcePrefix
	inst, err := ec2.NewInstance(ctx, resourcePrefix+name, &ec2.InstanceArgs{
		InstanceType:             pulumi.String(instanceType),
		Ami:                      pulumi.String(amiID),
		SubnetId:                 pulumi.String(subnetID),
		AssociatePublicIpAddress: pulumi.Bool(true),
		DisableApiTermination:    pulumi.Bool(config.Require("debug") != "true"),
	})
	if err != nil {
		return nil, err
	}

	for n, sg := range securityGroups {
		// Attach the security group to the instance
		_, err = ec2.NewNetworkInterfaceSecurityGroupAttachment(
			ctx,
			fmt.Sprintf("%s%s-%s-sga", resourcePrefix, name, n),
			&ec2.NetworkInterfaceSecurityGroupAttachmentArgs{
				NetworkInterfaceId: inst.PrimaryNetworkInterfaceId,
				SecurityGroupId:    sg.ID(),
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return inst, nil
}
