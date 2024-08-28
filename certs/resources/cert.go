package resources

import (
	clusterResources "copr-pulumi-go-aws/resources"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/acm"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateCert(
	ctx *pulumi.Context,
	cfg *config.Config,
	name string,
	primaryDomain string,
	sans []string,
) (*acm.Certificate, error) {
	resourcePrefix := cfg.Require("resourcePrefix")
	domainName := cfg.Require("albDomainName")
	zoneId, err := clusterResources.GetPublicHostedZoneID(ctx, cfg.Require("vpcProjectName"))

	cert, err := acm.NewCertificate(ctx, resourcePrefix+name, &acm.CertificateArgs{
		DomainName:              pulumi.String(domainName),
		SubjectAlternativeNames: pulumi.ToStringArray(sans),
		ValidationMethod:        pulumi.String("DNS"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String(resourcePrefix + name),
		},
	})
	if err != nil {
		return nil, err
	}

	validationNames, _ := cert.DomainValidationOptions.ApplyT(func(dvoA []acm.CertificateDomainValidationOption) ([]string, error) {
		_crtNames := []string{}
		for _, dvo := range dvoA {
			_crtNames = append(_crtNames, *dvo.ResourceRecordName)

			_, err = route53.NewRecord(ctx,
				fmt.Sprintf("%s%s-validation-record-%s", resourcePrefix, name, *dvo.ResourceRecordName),
				&route53.RecordArgs{
					ZoneId:         zoneId,
					AllowOverwrite: pulumi.Bool(true),

					Name:    pulumi.String(*dvo.ResourceRecordName),
					Type:    pulumi.String(*dvo.ResourceRecordType),
					Ttl:     pulumi.Int(60),
					Records: pulumi.StringArray{pulumi.String(*dvo.ResourceRecordValue)},
				})
			if err != nil {
				return nil, err
			}
		}
		return _crtNames, nil
	}).(pulumi.StringArrayOutput)

	cv, err := acm.NewCertificateValidation(ctx, resourcePrefix+name+"-validation", &acm.CertificateValidationArgs{
		CertificateArn:        cert.Arn,
		ValidationRecordFqdns: validationNames,
	})

	ctx.Export("certArn"+name, cert.Arn)
	ctx.Export("certDomainName"+name, cert.DomainName)
	ctx.Export("certSANs"+name, cert.SubjectAlternativeNames)
	ctx.Export("certValidationNames"+name, validationNames)
	ctx.Export("certValidationDomainNames"+name, cv.ValidationRecordFqdns)

	return cert, nil
}
