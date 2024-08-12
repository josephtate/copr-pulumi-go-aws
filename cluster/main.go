package main

import (
	"copr-pulumi-go-aws/resources"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func backendSGs(cfg *config.Config, sGroups *resources.SecurityGroups) []*ec2.SecurityGroup {
	sgs := []*ec2.SecurityGroup{sGroups.Backend, sGroups.Internal}
	if !cfg.GetBool("provisionStandaloneFrontend") {
		sgs = append(sgs, sGroups.Frontend)
	}
	return sgs
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Fetch configuration values
		cfg := config.New(ctx, "copr-pulumi-go-aws")

		sGroups, err := resources.CreateSecurityGroups(ctx, cfg)
		if err != nil {
			return err
		}

		// Please note that "backend" is named such because it is the executing engine of COPR, not the due to the
		// typical frontend/backend development architecture. It is not the backend of the application.
		_, err = resources.CreateInstance(ctx, cfg, "backend", cfg.GetInt("instanceRootVolSizeBackend"), backendSGs(cfg, sGroups), true)
		if err != nil {
			return err
		}

		if config.GetBool(ctx, "provisionStandaloneFrontend") {
			rootSize := cfg.GetInt("instanceRootVolSizeBackend")
			if rootSize == 0 {
				rootSize = 30
			}
			_, err = resources.CreateInstance(ctx, cfg, "frontend",
				rootSize,
				[]*ec2.SecurityGroup{
					sGroups.Frontend,
					sGroups.Internal,
				}, true)
			if err != nil {
				return err
			}
		}

		if config.GetBool(ctx, "provisionStandaloneDistGit") {
			rootSize := cfg.GetInt("instanceRootVolSizeBackend")
			if rootSize == 0 {
				rootSize = 30
			}
			_, err = resources.CreateInstance(ctx, cfg, "distgit", rootSize, []*ec2.SecurityGroup{
				sGroups.DistGit,
				sGroups.Internal,
			}, true)
			if err != nil {
				return err
			}
		}

		if config.GetBool(ctx, "provisionStandaloneKeyGen") {
			rootSize := cfg.GetInt("instanceRootVolSizeBackend")
			if rootSize == 0 {
				rootSize = 30
			}
			_, err = resources.CreateInstance(ctx, cfg, "keygen", rootSize, []*ec2.SecurityGroup{
				sGroups.KeyGen,
				sGroups.Internal,
			}, true)
			if err != nil {
				return err
			}
		}

		if config.GetBool(ctx, "provisionStandaloneDB") {
			err = resources.CreateDatabase(ctx, cfg, sGroups.DB)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
