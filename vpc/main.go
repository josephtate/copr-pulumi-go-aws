package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "copr-pulumi-go-aws-vpc")

		resPrefix := cfg.Require("ResourcePrefix")
		VPCCIDR := cfg.Require("VPCCIDR")
		privateDomain := cfg.Require("PrivateDomainName")
		publicDomain := cfg.Require("PublicDomainName")

		// r, _ := aws.GetRegion(ctx, nil, nil)
		// region := r.Name
		azs, _ := aws.GetAvailabilityZones(ctx, nil, nil)
		numAZs := len(azs.Names)

		var publicSubnetCidrBlocks []string
		var privateSubnetCidrBlocks []string
		cfg.RequireObject("PublicSubnetCIDRs", &publicSubnetCidrBlocks)
		cfg.RequireObject("PrivateSubnetCIDRs", &privateSubnetCidrBlocks)

		vpc, err := ec2.NewVpc(ctx, resPrefix+"vpc", &ec2.VpcArgs{
			CidrBlock:          pulumi.String(VPCCIDR),
			EnableDnsSupport:   pulumi.Bool(true),
			EnableDnsHostnames: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		internetGateway, err := ec2.NewInternetGateway(ctx, resPrefix+"igw", &ec2.InternetGatewayArgs{
			VpcId: vpc.ID(),
		})
		if err != nil {
			return err
		}

		routeTable, err := ec2.NewRouteTable(ctx, resPrefix+"public-rt", &ec2.RouteTableArgs{
			VpcId: vpc.ID(),
			Routes: ec2.RouteTableRouteArray{
				&ec2.RouteTableRouteArgs{
					CidrBlock: pulumi.String("0.0.0.0/0"),
					GatewayId: internetGateway.ID(),
				},
			},
		})
		if err != nil {
			return err
		}

		publicSubnets := make([]*ec2.Subnet, len(publicSubnetCidrBlocks))
		privateSubnets := make([]*ec2.Subnet, len(privateSubnetCidrBlocks))
		natGateways := make(map[string]*ec2.NatGateway, len(publicSubnetCidrBlocks))

		for i, publicSubnetCidrBlock := range publicSubnetCidrBlocks {
			// create subnets in azs back to front
			az := azs.Names[numAZs-(1+i%numAZs)]

			pSN := fmt.Sprintf("%spublic-subnet-%s", resPrefix, az)
			publicSubnet, err := ec2.NewSubnet(ctx, pSN, &ec2.SubnetArgs{
				VpcId:               vpc.ID(),
				CidrBlock:           pulumi.String(publicSubnetCidrBlock),
				AvailabilityZone:    pulumi.String(az),
				MapPublicIpOnLaunch: pulumi.Bool(true),
			})
			if err != nil {
				return err
			}

			pRTAN := fmt.Sprintf("%spublic-rta-%s", resPrefix, az)
			_, err = ec2.NewRouteTableAssociation(ctx, pRTAN, &ec2.RouteTableAssociationArgs{
				SubnetId:     publicSubnet.ID(),
				RouteTableId: routeTable.ID(),
			})
			if err != nil {
				return err
			}

			if natGateways[az] == nil {
				eip, err := ec2.NewEip(ctx, fmt.Sprintf("%seip-%s", resPrefix, az), nil)
				if err != nil {
					return err
				}

				pNGN := fmt.Sprintf("%snat-gateway-%s", resPrefix, az)
				natGateway, err := ec2.NewNatGateway(ctx, pNGN, &ec2.NatGatewayArgs{
					SubnetId:     publicSubnet.ID(),
					AllocationId: eip.ID(),
				})
				if err != nil {
					return err
				}

				natGateways[az] = natGateway
			}
			publicSubnets[i] = publicSubnet
		}

		for i, privateSubnetCidrBlock := range privateSubnetCidrBlocks {
			az := azs.Names[numAZs-(1+i%numAZs)]

			prSN := fmt.Sprintf("%sprivate-subnet-%s", resPrefix, az)
			privateSubnet, err := ec2.NewSubnet(ctx, prSN, &ec2.SubnetArgs{
				VpcId:            vpc.ID(),
				CidrBlock:        pulumi.String(privateSubnetCidrBlock),
				AvailabilityZone: pulumi.String(az),
			})
			if err != nil {
				return err
			}

			prRTAN := fmt.Sprintf("%sprivate-rta-%s", resPrefix, az)
			if natGateways[az] != nil {
				privateRouteTable, err := ec2.NewRouteTable(ctx, prRTAN, &ec2.RouteTableArgs{
					VpcId: vpc.ID(),
					Routes: ec2.RouteTableRouteArray{
						&ec2.RouteTableRouteArgs{
							CidrBlock:    pulumi.String("0.0.0.0/0"),
							NatGatewayId: natGateways[az].ID(),
						},
					},
				})
				if err != nil {
					return err
				}

				prRTAN := fmt.Sprintf("%sprivate-rta-%s", resPrefix, az)
				_, err = ec2.NewRouteTableAssociation(ctx, prRTAN, &ec2.RouteTableAssociationArgs{
					SubnetId:     privateSubnet.ID(),
					RouteTableId: privateRouteTable.ID(),
				})
				if err != nil {
					return err
				}

			}

			privateSubnets[i] = privateSubnet
		}

		// Lookup the public hosted zone

		print("The Public Domain: ", publicDomain)
		publicHostedZone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
			Name: pulumi.StringRef(publicDomain + "."),
		})
		if err != nil {
			return err
		}

		privateHostedZone, err := route53.NewZone(ctx, resPrefix+"private-hosted-zone", &route53.ZoneArgs{
			Name: pulumi.String(privateDomain + "."),
			Vpcs: route53.ZoneVpcArray{
				&route53.ZoneVpcArgs{
					VpcId: vpc.ID(),
				},
			},
		})
		if err != nil {
			return err
		}

		// Export the hosted zone ID
		ctx.Export("publicHostedZoneId", pulumi.String(publicHostedZone.ZoneId))
		ctx.Export("privateHostedZoneId", privateHostedZone.ID())

		ctx.Export("publicDomainName", pulumi.String(publicDomain))
		ctx.Export("privateDomainName", pulumi.String(privateDomain))

		ctx.Export("vpcId", vpc.ID())
		for i, subnet := range publicSubnets {
			ctx.Export(fmt.Sprintf("publicSubnet%d", i), subnet.ID())
		}
		for i, subnet := range privateSubnets {
			ctx.Export(fmt.Sprintf("privateSubnet%d", i), subnet.ID())
		}

		return nil
	})
}
