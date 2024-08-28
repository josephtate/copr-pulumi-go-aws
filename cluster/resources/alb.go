package resources

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/alb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateALB(
	ctx *pulumi.Context,
	cfg *config.Config,
	name string,
	listenerInstances []*ec2.Instance,
	securityGroups []*ec2.SecurityGroup,
	public bool,
) (*alb.LoadBalancer, error) {
	// Fetch configuration values
	resourcePrefix := cfg.Require("resourcePrefix")

	fqdns, _ := GetALBCertSubjectAlternativeNames(ctx, cfg.Require("certsProjectName"))

	vpcid, err := GetVPCID(ctx, cfg.Require("vpcProjectName"))

	subnets, err := GetSubnets(ctx, cfg.Require("vpcProjectName"), public)
	if err != nil {
		return nil, err
	}

	ids := make(pulumi.StringArray, len(securityGroups))
	for i, sgid := range securityGroups {
		ids[i] = sgid.ID()
	}

	// Create the ALB
	loadBalancer, err := alb.NewLoadBalancer(ctx, resourcePrefix+name, &alb.LoadBalancerArgs{
		Subnets:        subnets,
		SecurityGroups: ids,
	})
	if err != nil {
		return nil, err
	}

	zoneId, _ := GetPublicHostedZoneID(ctx, cfg.Require("vpcProjectName"))

	r53Names := fqdns.ApplyT(func(values []string) ([]string, error) {
		_r53Names := []string{}
		for _, fqdn := range values {
			_r53Names = append(_r53Names, fqdn)
			_, err := route53.NewRecord(ctx, resourcePrefix+name+fmt.Sprintf("-dns-%s", fqdn), &route53.RecordArgs{
				ZoneId:         zoneId,
				Name:           pulumi.String(fqdn),
				Type:           pulumi.String("A"),
				AllowOverwrite: pulumi.Bool(true),
				Aliases: route53.RecordAliasArray{
					&route53.RecordAliasArgs{
						Name:                 loadBalancer.DnsName,
						ZoneId:               loadBalancer.ZoneId,
						EvaluateTargetHealth: pulumi.Bool(false),
					},
				},
			})
			if err != nil {
				return nil, err
			}
		}
		return _r53Names, nil
	}).(pulumi.StringArrayOutput)

	ctx.Export("albDNSNames", r53Names)

	//Target Group
	var targetGroup *alb.TargetGroup
	targetGroup, err = alb.NewTargetGroup(ctx, resourcePrefix+name+"-tg", &alb.TargetGroupArgs{
		Port:            pulumi.Int(5000),
		Protocol:        pulumi.String("HTTP"),
		ProtocolVersion: pulumi.String("HTTP1"),
		TargetType:      pulumi.String("instance"),
		VpcId:           vpcid,
		HealthCheck: &alb.TargetGroupHealthCheckArgs{
			Enabled:          pulumi.Bool(true),
			HealthyThreshold: pulumi.Int(2),
			Interval:         pulumi.Int(10),
			Matcher:          pulumi.String("200"),
			Path:             pulumi.String("/rss/"),
		},
	})
	if err != nil {
		return nil, err
	}

	for _, instance := range listenerInstances {
		_, err = alb.NewTargetGroupAttachment(ctx, resourcePrefix+name+"-frontend-tga", &alb.TargetGroupAttachmentArgs{
			TargetGroupArn: targetGroup.Arn,
			TargetId:       instance.ID(),
			Port:           pulumi.Int(5000),
		})
		if err != nil {
			return nil, err
		}
	}

	_, err = alb.NewListener(ctx, resourcePrefix+name+"-http-redirect-listener", &alb.ListenerArgs{
		LoadBalancerArn: loadBalancer.Arn,
		Port:            pulumi.Int(80),
		Protocol:        pulumi.String("HTTP"),
		DefaultActions: alb.ListenerDefaultActionArray{
			&alb.ListenerDefaultActionArgs{
				Type: pulumi.String("redirect"),
				Redirect: &alb.ListenerDefaultActionRedirectArgs{
					StatusCode: pulumi.String("HTTP_301"),
					Protocol:   pulumi.String("HTTPS"),
					Port:       pulumi.String("443"),
				},
			},
		},
	})

	// Get the ALB Cert ARN
	certArn, err := GetALBCertARN(ctx, cfg.Require("certsProjectName"))

	_, err = alb.NewListener(ctx, resourcePrefix+name+"-https-forward-listener", &alb.ListenerArgs{
		LoadBalancerArn: loadBalancer.Arn,
		Port:            pulumi.Int(443),
		Protocol:        pulumi.String("HTTPS"),
		SslPolicy:       pulumi.String("ELBSecurityPolicy-2016-08"),
		CertificateArn:  certArn,
		DefaultActions: alb.ListenerDefaultActionArray{
			&alb.ListenerDefaultActionArgs{
				Type: pulumi.String("forward"),
				Forward: &alb.ListenerDefaultActionForwardArgs{
					TargetGroups: alb.ListenerDefaultActionForwardTargetGroupArray{
						&alb.ListenerDefaultActionForwardTargetGroupArgs{
							Arn: targetGroup.Arn,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	ctx.Export("albDNS", loadBalancer.DnsName)

	return loadBalancer, nil

}
