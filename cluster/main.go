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

		beinst, err := resources.CreateInstance(ctx, cfg, "backend", map[string]*ec2.SecurityGroup{
			"backend":  sGroups.Backend,
			"internal": sGroups.Internal,
		}, false)
		if err != nil {
			return err
		}

		if config.GetBool(ctx, "provisionSeparateFrontend") {
			_, err = resources.CreateInstance(ctx, cfg, "frontend", map[string]*ec2.SecurityGroup{
				"frontend": sGroups.Frontend,
				"internal": sGroups.Internal,
			}, true)
			if err != nil {
				return err
			}
		} else {
			// Add the fe SG to the backend
			resources.AttachSecurityGroups(ctx, cfg, "backend", beinst, map[string]*ec2.SecurityGroup{
				"frontend": sGroups.Frontend})
		}

		if config.GetBool(ctx, "provisionSeparateDistGit") {

			_, err = resources.CreateInstance(ctx, cfg, "distgit", map[string]*ec2.SecurityGroup{
				"distgit":  sGroups.DistGit,
				"internal": sGroups.Internal,
			}, true)
			if err != nil {
				return err
			}
		} else {
			// Add the distgit SG to the backend
			resources.AttachSecurityGroups(ctx, cfg, "backend", beinst, map[string]*ec2.SecurityGroup{
				"distgit": sGroups.DistGit})
		}

		if config.GetBool(ctx, "provisionSeparateKeyGen") {
			_, err = resources.CreateInstance(ctx, cfg, "keygen", map[string]*ec2.SecurityGroup{
				"keygen":   sGroups.KeyGen,
				"internal": sGroups.Internal,
			}, true)
			if err != nil {
				return err
			}
		} else {
			// Add the keygen SG to the backend
			resources.AttachSecurityGroups(ctx, cfg, "backend", beinst, map[string]*ec2.SecurityGroup{
				"keygen": sGroups.KeyGen})

		}

		if config.GetBool(ctx, "provisionSeparateDB") {
			err = resources.CreateDatabase(ctx, cfg, sGroups.DB)
			if err != nil {
				return err
			}
		} else {
			resources.AttachSecurityGroups(ctx, cfg, "backend", beinst, map[string]*ec2.SecurityGroup{
				"db": sGroups.DB})
		}
		return nil
	})
}
