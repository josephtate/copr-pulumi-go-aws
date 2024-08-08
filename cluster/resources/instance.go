package resources

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ssm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

var sshKeyCache sync.Map

// SetupSSHKey sets up the SSH key for the instances using Pulumi and memoizes the result.
func SetupSSHKey(ctx *pulumi.Context, name, ssmPath string) (*ec2.KeyPair, error) {
	if key, ok := sshKeyCache.Load(ssmPath); ok {
		return key.(*ec2.KeyPair), nil
	}

	// Will break if run from Windows
	publicKeyParam := filepath.Join(ssmPath, "public-key")

	publicKey, err := ssm.LookupParameter(ctx, &ssm.LookupParameterArgs{
		Name: publicKeyParam,
	})
	if err != nil {
		return nil, err
	}

	sshKey, err := ec2.NewKeyPair(ctx, name, &ec2.KeyPairArgs{
		KeyNamePrefix: pulumi.String(name),
		PublicKey:     pulumi.String(publicKey.Value),
	})
	if err != nil {
		return nil, err
	}

	sshKeyCache.Store(ssmPath, sshKey)
	return sshKey, nil
}

func AttachSecurityGroups(
	ctx *pulumi.Context,
	config *config.Config,
	name string,
	inst *ec2.Instance,
	securityGroups map[string]*ec2.SecurityGroup,
) error {
	resourcePrefix := config.Require("resourcePrefix")

	for n, sg := range securityGroups {
		// Attach the security group to the instance
		_, err := ec2.NewNetworkInterfaceSecurityGroupAttachment(
			ctx,
			fmt.Sprintf("%s%s-%s-sga", resourcePrefix, name, n),
			&ec2.NetworkInterfaceSecurityGroupAttachmentArgs{
				NetworkInterfaceId: inst.PrimaryNetworkInterfaceId,
				SecurityGroupId:    sg.ID(),
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Please note that "backend" is named such because it is the executing engine of COPR, not the due to the typical
// frontend/backend development architecture. It is not the backend of the application.
func CreateInstance(
	ctx *pulumi.Context,
	config *config.Config,
	name string,
	securityGroups map[string]*ec2.SecurityGroup,
	public bool,
) (*ec2.Instance, error) {
	resourcePrefix := config.Require("resourcePrefix")
	sshKeyPath := config.Require("sshKeySSMPathBase")
	instanceType := config.Require("instanceTypeBackend")
	vpcProject := config.Require("vpcProjectName")

	subnetID, err := GetFirstSubnet(ctx, vpcProject, public)
	if err != nil {
		return nil, err
	}

	amiID, err := GetLatestRocky9Ami(ctx)
	if err != nil {
		return nil, err
	}
	// Launch an EC2 instance with the resourcePrefix
	sshKey, err := SetupSSHKey(ctx, resourcePrefix+"keypair", sshKeyPath)
	if err != nil {
		return nil, err
	}

	inst, err := ec2.NewInstance(ctx, resourcePrefix+name, &ec2.InstanceArgs{
		InstanceType:             pulumi.String(instanceType),
		Ami:                      pulumi.String(amiID),
		SubnetId:                 subnetID,
		AssociatePublicIpAddress: pulumi.Bool(public),
		DisableApiTermination:    pulumi.Bool(config.RequireBool("debug")),
		KeyName:                  sshKey.KeyName,
	})
	if err != nil {
		return nil, err
	}

	err = AttachSecurityGroups(ctx, config, name, inst, securityGroups)

	return inst, nil
}
