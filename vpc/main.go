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
			Tags: pulumi.StringMap{
				"Name": pulumi.String(resPrefix + "vpc"),
			},
		})
		if err != nil {
			return err
		}

		internetGateway, err := ec2.NewInternetGateway(ctx, resPrefix+"igw", &ec2.InternetGatewayArgs{
			VpcId: vpc.ID(),
			Tags: pulumi.StringMap{
				"Name": pulumi.String(resPrefix + "igw"),
			},
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

		publicSubnetsByAz := make(map[string]*ec2.Subnet, len(publicSubnetCidrBlocks))
		privateSubnetsByAz := make(map[string]*ec2.Subnet, len(privateSubnetCidrBlocks))

		for i, publicSubnetCidrBlock := range publicSubnetCidrBlocks {
			// create subnets in azs back to front
			az := azs.Names[numAZs-(1+i%numAZs)]

			pSN := fmt.Sprintf("%spublic-subnet-%s", resPrefix, az)
			publicSubnet, err := ec2.NewSubnet(ctx, pSN, &ec2.SubnetArgs{
				VpcId:               vpc.ID(),
				CidrBlock:           pulumi.String(publicSubnetCidrBlock),
				AvailabilityZone:    pulumi.String(az),
				MapPublicIpOnLaunch: pulumi.Bool(true),
				Tags: pulumi.StringMap{
					"Name": pulumi.String(pSN),
				},
			})
			if err != nil {
				return err
			}
			publicSubnetsByAz[az] = publicSubnet

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
					Tags: pulumi.StringMap{
						"Name": pulumi.String(pNGN),
					},
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
				Tags: pulumi.StringMap{
					"Name": pulumi.String(prSN),
				},
			})
			if err != nil {
				return err
			}

			privateSubnetsByAz[az] = privateSubnet

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

		nets := make(pulumi.StringArray, len(publicSubnets))
		for i, subnet := range publicSubnets {
			nets[i] = subnet.ID().ToStringOutput()
		}

		mnets := make(pulumi.StringMap, len(publicSubnets))
		for az, subnet := range publicSubnetsByAz {
			mnets[az] = subnet.ID().ToStringOutput()
		}
		ctx.Export("publicSubnets", nets.ToStringArrayOutput())
		ctx.Export("publicSubnetsAZs", mnets.ToStringMapOutput())

		nets2 := make(pulumi.StringArray, len(privateSubnets))
		for i, subnet := range privateSubnets {
			nets2[i] = subnet.ID().ToStringOutput()
		}

		mnets2 := make(pulumi.StringMap, len(privateSubnets))
		for az, subnet := range privateSubnetsByAz {
			mnets2[az] = subnet.ID().ToStringOutput()
		}

		ctx.Export("privateSubnets", nets2.ToStringArrayOutput())
		ctx.Export("privateSubnetsAZs", mnets2.ToStringMapOutput())

		return nil
	})
}
