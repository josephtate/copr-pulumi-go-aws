package main

import (
	"copr-pulumi-go-aws-certs/resources"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Fetch configuration values
		cfg := config.New(ctx, "copr-pulumi-go-aws-certs")

		cert, err := resources.CreateCert(ctx, cfg, "alb-cert", cfg.Require("albDomainName"), resources.GetALBSANs(cfg))
		if err != nil {
			return err
		}

		ctx.Export("ALBCertARN", cert.Arn)
		ctx.Export("ALBCertDomainName", cert.DomainName)
		ctx.Export("ALBCertSubjectAlternativeNames", cert.SubjectAlternativeNames)
		return nil
	})
}
