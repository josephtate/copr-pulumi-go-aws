package resources

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
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

func Route53Record(ctx *pulumi.Context, name string, zoneID, hostname, ip *pulumi.StringOutput, ttl int) (*route53.Record, error) {
	return route53.NewRecord(ctx, name, &route53.RecordArgs{
		Name:    hostname,
		ZoneId:  zoneID,
		Type:    pulumi.String("A"),
		Ttl:     pulumi.Int(ttl),
		Records: pulumi.StringArray{ip},
	})
}

func CreateInstance(
	ctx *pulumi.Context,
	cfg *config.Config,
	name string,
	securityGroups []*ec2.SecurityGroup,
	public bool,
) (*ec2.Instance, error) {
	resourcePrefix := cfg.Require("resourcePrefix")
	sshKeyPath := cfg.Require("sshKeySSMPathBase")
	instanceType := cfg.Require("instanceTypeBackend")
	vpcProject := cfg.Require("vpcProjectName")
	userSSHKeys := getAdminSSHKeys(cfg)

	subnetID, err := GetFirstSubnet(ctx, vpcProject, public)
	if err != nil {
		return nil, err
	}

	amiID, err := GetLatestRocky9Ami(ctx)
	if err != nil {
		return nil, err
	}

	sshKey, err := SetupSSHKey(ctx, resourcePrefix+"keypair", sshKeyPath)
	if err != nil {
		return nil, err
	}

	extDomain, err := GetPublicDomainName(ctx, vpcProject)
	if err != nil {
		return nil, err
	}

	intDomain, err := GetPrivateDomainName(ctx, vpcProject)
	if err != nil {
		return nil, err
	}

	intZoneID, err := GetPrivateHostedZoneID(ctx, vpcProject)
	extZoneID, err := GetPublicHostedZoneID(ctx, vpcProject)

	subDomain := strings.TrimSuffix(resourcePrefix, "-")
	subDomain = strings.ReplaceAll(subDomain, "-", ".")
	intHostname := pulumi.Sprintf("%s.%s.%s", name, subDomain, intDomain)
	extHostname := pulumi.Sprintf("%s.%s.%s", name, subDomain, extDomain)
	var userData pulumi.StringOutput
	if len(userSSHKeys) > 0 {
		userData = pulumi.Sprintf(`#cloud-config
hostname: %s
users:
  - name: rocky
    ssh-authorized-keys:
    - %s
`,
			intHostname,
			strings.Join(userSSHKeys, "\n    - "))
	} else {
		userData = pulumi.String("").ToStringOutput()
	}
	debug := cfg.RequireBool("debug")

	defaultTags := getDefaultTags(cfg)

	// Convert default tags to pulumi.StringMap
	tags := pulumi.StringMap{}
	for k, v := range defaultTags {
		tags[k] = pulumi.String(v)
	}

	tags["Name"] = pulumi.String(resourcePrefix + name)
	tags["ansible-ssh-user"] = pulumi.String("rocky")
	tags["ansible-python-interpreter"] = pulumi.String("/usr/bin/python3")

	ids := make(pulumi.StringArray, len(securityGroups))
	for i, sgid := range securityGroups {
		ids[i] = sgid.ID()
	}

	// Launch an EC2 instance with the resourcePrefix
	inst, err := ec2.NewInstance(ctx, resourcePrefix+name, &ec2.InstanceArgs{
		InstanceType:             pulumi.String(instanceType),
		Ami:                      pulumi.String(amiID),
		SubnetId:                 subnetID,
		AssociatePublicIpAddress: pulumi.Bool(public),
		DisableApiTermination:    pulumi.Bool(debug),
		KeyName:                  sshKey.KeyName,
		VpcSecurityGroupIds:      ids,
		UserData:                 userData,
		Tags:                     tags,
	})
	if err != nil {
		return nil, err
	}

	ttl := 300
	if debug {
		ttl = 60
	}

	_, err = Route53Record(ctx, resourcePrefix+name+"-int-route53-record", &intZoneID, &intHostname, &inst.PrivateIp, ttl)
	if err != nil {
		return nil, err
	}

	if public {
		_, err = Route53Record(ctx, resourcePrefix+name+"-ext-route53-record", &extZoneID, &extHostname, &inst.PublicIp, ttl)
		if err != nil {
			return nil, err
		}
		ctx.Export(name+"InstancePublicIP", inst.PublicIp)
		ctx.Export(name+"InstancePublicDNS", inst.PublicDns)
		ctx.Export(name+"InstancePublicHostname", extHostname)
	}

	// err = AttachSecurityGroups(ctx, cfg, name, inst, securityGroups)

	ctx.Export(name+"InstancePrivateIP", inst.PrivateIp)
	ctx.Export(name+"InstancePrivateDNS", inst.PrivateDns)
	ctx.Export(name+"InstancePrivateHostname", intHostname)

	ctx.Export(name+"InstanceSSHKeyPair", sshKey.KeyName)
	return inst, nil
}
