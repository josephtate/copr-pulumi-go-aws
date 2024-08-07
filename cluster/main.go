package main

import (
	"copr-pulumi-go-aws/resources"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Fetch configuration values
		cfg := config.New(ctx, "copr-pulumi-go-aws")

		sGroups, err := resources.CreateSecurityGroups(ctx, cfg)
		if err != nil {
			return err
		}

		_, err = resources.CreateInstance(ctx, cfg, "backend", map[string]*ec2.SecurityGroup{
			"backend":  sGroups.Backend,
			"internal": sGroups.Internal,
		})
		if err != nil {
			return err
		}

		_, err = resources.CreateInstance(ctx, cfg, "frontend", map[string]*ec2.SecurityGroup{
			"frontend": sGroups.Frontend,
			"internal": sGroups.Internal,
		})
		if err != nil {
			return err
		}

		_, err = resources.CreateInstance(ctx, cfg, "distgit", map[string]*ec2.SecurityGroup{
			"distgit":  sGroups.DistGit,
			"internal": sGroups.Internal,
		})
		if err != nil {
			return err
		}

		_, err = resources.CreateInstance(ctx, cfg, "keygen", map[string]*ec2.SecurityGroup{
			"keygen":   sGroups.KeyGen,
			"internal": sGroups.Internal,
		})
		if err != nil {
			return err
		}

		err = resources.CreateDatabase(ctx, cfg, sGroups.DB)
		if err != nil {
			return err
		}

		return nil
	})
}
