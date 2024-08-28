package resources

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/vpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type SecurityGroups struct {
	Internal *ec2.SecurityGroup
	Backend  *ec2.SecurityGroup
	Frontend *ec2.SecurityGroup
	DistGit  *ec2.SecurityGroup
	KeyGen   *ec2.SecurityGroup
	LB       *ec2.SecurityGroup
	DB       *ec2.SecurityGroup
	Builder  *ec2.SecurityGroup
}

func CreateSecurityGroups(ctx *pulumi.Context, config *config.Config) (*SecurityGroups, error) {
	resourcePrefix := config.Require("resourcePrefix")
	vpcID, err := GetVPCID(ctx, config.Require("vpcProjectName"))
	if err != nil {
		return nil, err
	}

	// debug := config.RequireBool("debug")

	isg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"internal-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpcID,
		Name:        pulumi.String(resourcePrefix + "internal-sg"),
		Description: pulumi.String("Assigned to all instances: Allows all traffic between instances in the cluster. This is a /get things done/ SG. Please replace this with individual rules for each machine type."),

		Tags: pulumi.StringMap{
			"Name": pulumi.String(resourcePrefix + "internal-sg"),
		},
	})
	if err != nil {
		return nil, err
	}

	_, err = vpc.NewSecurityGroupIngressRule(ctx, resourcePrefix+"internal-all-ingress-from-internal-sgr", &vpc.SecurityGroupIngressRuleArgs{
		Description:               pulumi.String("Allow all cross traffic for the cluster"),
		SecurityGroupId:           isg.ID(),
		ReferencedSecurityGroupId: isg.ID(),
		FromPort:                  pulumi.Int(0),
		ToPort:                    pulumi.Int(0),
		IpProtocol:                pulumi.String("-1"),
	})
	if err != nil {
		return nil, err
	}

	_, err = vpc.NewSecurityGroupEgressRule(ctx, resourcePrefix+"internal-all-ipv4-egress-sgr", &vpc.SecurityGroupEgressRuleArgs{
		Description:     pulumi.String("Allow all traffic out on the internal SG"),
		SecurityGroupId: isg.ID(),
		IpProtocol:      pulumi.String("-1"),
		FromPort:        pulumi.Int(0),
		ToPort:          pulumi.Int(0),
		CidrIpv4:        pulumi.String("0.0.0.0/0"),
	})
	if err != nil {
		return nil, err
	}

	_, err = vpc.NewSecurityGroupEgressRule(ctx, resourcePrefix+"internal-all-ipv6-egress-sgr", &vpc.SecurityGroupEgressRuleArgs{
		Description:     pulumi.String("Allow all traffic out on the internal SG"),
		SecurityGroupId: isg.ID(),
		IpProtocol:      pulumi.String("-1"),
		FromPort:        pulumi.Int(0),
		ToPort:          pulumi.Int(0),
		CidrIpv6:        pulumi.String("::/0"),
	})

	if err != nil {
		return nil, err
	}

	// Create a security group with the resourcePrefix
	besg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"backend-sg", &ec2.SecurityGroupArgs{
		VpcId: vpcID,
		Name:  pulumi.String(resourcePrefix + "backend-sg"),
	})
	if err != nil {
		return nil, err
	}

	fesg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"frontend-sg", &ec2.SecurityGroupArgs{
		VpcId: vpcID,
		Name:  pulumi.String(resourcePrefix + "frontend-sg"),
	})
	if err != nil {
		return nil, err
	}

	dgsg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"distgit-sg", &ec2.SecurityGroupArgs{
		VpcId: vpcID,
		Name:  pulumi.String(resourcePrefix + "distgit-sg"),
	})
	if err != nil {
		return nil, err
	}

	kgsg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"keygen-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpcID,
		Name:        pulumi.String(resourcePrefix + "keygen-sg"),
		Description: pulumi.String("Assigned to the keygen instances, only allows traffic from backend"),
	})
	if err != nil {
		return nil, err
	}

	lbsg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"lb-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpcID,
		Name:        pulumi.String(resourcePrefix + "lb-sg"),
		Description: pulumi.String("Assigned to the load balancer to allow traffic"),
	})
	if err != nil {
		return nil, err
	}

	// Allow traffic only to ports 80 and 443 of load balancer
	for _, port := range []int{80, 443} {
		_, err = vpc.NewSecurityGroupIngressRule(ctx, fmt.Sprintf(resourcePrefix+"lb-%d-v4-ingress-from-world", port),
			&vpc.SecurityGroupIngressRuleArgs{
				Description:     pulumi.String("Allow IPv4 traffic from the world to the load balancer"),
				SecurityGroupId: lbsg.ID(),
				IpProtocol:      pulumi.String("tcp"),
				FromPort:        pulumi.Int(port),
				ToPort:          pulumi.Int(port),
				CidrIpv4:        pulumi.String("0.0.0.0/0"),
			})
		_, err = vpc.NewSecurityGroupIngressRule(ctx, fmt.Sprintf(resourcePrefix+"lb-%d-v6-ingress-from-world", port),
			&vpc.SecurityGroupIngressRuleArgs{
				Description:     pulumi.String("Allow IPv6 traffic from the world to the load balancer"),
				SecurityGroupId: lbsg.ID(),
				IpProtocol:      pulumi.String("tcp"),
				FromPort:        pulumi.Int(port),
				ToPort:          pulumi.Int(port),
				CidrIpv6:        pulumi.String("::/0"),
			})
	}
	// allow lb traffic outbound to frontend group
	_, err = vpc.NewSecurityGroupEgressRule(ctx, resourcePrefix+"lb-egress-to-frontend-sg", &vpc.SecurityGroupEgressRuleArgs{
		Description:               pulumi.String("Allow traffic from the load balancer to the frontend"),
		SecurityGroupId:           lbsg.ID(),
		ReferencedSecurityGroupId: fesg.ID(),
		IpProtocol:                pulumi.String("tcp"),
		FromPort:                  pulumi.Int(5000),
		ToPort:                    pulumi.Int(5000),
	})
	if err != nil {
		return nil, err
	}

	// Allow lb traffic to the frontend group
	_, err = vpc.NewSecurityGroupIngressRule(ctx, resourcePrefix+"frontend-ingress-from-lb-sg", &vpc.SecurityGroupIngressRuleArgs{
		Description:               pulumi.String("Allow traffic from the load balancer to the frontend"),
		SecurityGroupId:           fesg.ID(),
		ReferencedSecurityGroupId: lbsg.ID(),
		IpProtocol:                pulumi.String("tcp"),
		FromPort:                  pulumi.Int(5000),
		ToPort:                    pulumi.Int(5000),
	})
	if err != nil {
		return nil, err
	}

	dbsg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"db-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpcID,
		Name:        pulumi.String(resourcePrefix + "db-sg"),
		Description: pulumi.String("Assigned to all database instances, only allows traffic from authorized roles"),
	})
	if err != nil {
		return nil, err
	}

	_, err = vpc.NewSecurityGroupIngressRule(ctx, resourcePrefix+"db-ingress-from-frontend-sg", &vpc.SecurityGroupIngressRuleArgs{
		Description:               pulumi.String("Allow traffic from the frontend server"),
		SecurityGroupId:           dbsg.ID(),
		ReferencedSecurityGroupId: fesg.ID(),
		IpProtocol:                pulumi.String("tcp"),
		FromPort:                  pulumi.Int(5432),
		ToPort:                    pulumi.Int(5432),
	})
	if err != nil {
		return nil, err
	}

	buildersg, err := ec2.NewSecurityGroup(ctx, resourcePrefix+"builder-sg", &ec2.SecurityGroupArgs{
		VpcId:       vpcID,
		Name:        pulumi.String(resourcePrefix + "builder-sg"),
		Description: pulumi.String("Assigned to all builder instances"),
	})
	if err != nil {
		return nil, err
	}

	// Add SSH Ingress to the internal Security Group
	cidrs := getSSHCIDRs(config)

	for _, cidr := range cidrs {
		_, err := vpc.NewSecurityGroupIngressRule(
			ctx, resourcePrefix+"internal-ingress-ssh:"+cidr, &vpc.SecurityGroupIngressRuleArgs{
				Description:     pulumi.String("Allow SSH traffic from " + cidr),
				SecurityGroupId: isg.ID(),
				IpProtocol:      pulumi.String("-1"),
				CidrIpv4:        pulumi.String(cidr),
			})
		if err != nil {
			return nil, err
		}
	}

	return &SecurityGroups{isg, besg, fesg, dgsg, kgsg, lbsg, dbsg, buildersg}, nil
}
