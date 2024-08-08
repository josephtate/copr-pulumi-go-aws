package resources

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ssm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateDatabase(ctx *pulumi.Context, cfg *config.Config, dbsg *ec2.SecurityGroup) error {
	resourcePrefix := cfg.Require("resourcePrefix")
	instanceType := cfg.Require("instanceTypeDB")
	vpcProject := cfg.Require("vpcProjectName")
	debug := cfg.RequireBool("debug")

	subnetID, err := GetFirstSubnet(ctx, vpcProject, false)
	if err != nil {
		return err
	}

	ssmParameter, err := ssm.NewParameter(ctx, resourcePrefix+"db-password-parameter", &ssm.ParameterArgs{
		Name:        pulumi.String("/rds/" + resourcePrefix + "db-password"),
		Type:        pulumi.String("SecureString"),
		Value:       cfg.RequireSecret("dbPassword"),
		Description: pulumi.String("The admin password for the RDS instance"),
	})
	if err != nil {
		return err
	}

	// Create the RDS PostgreSQL instance
	db, err := rds.NewInstance(ctx, resourcePrefix+"db", &rds.InstanceArgs{
		InstanceClass:       pulumi.String(instanceType),
		AllocatedStorage:    pulumi.Int(20),
		Engine:              pulumi.String("postgres"),
		EngineVersion:       pulumi.String("15"),
		Username:            pulumi.String("admin"),
		Password:            ssmParameter.Value,
		DbSubnetGroupName:   subnetID,
		VpcSecurityGroupIds: pulumi.StringArray{dbsg.ID()},
		MaxAllocatedStorage: pulumi.Int(20),
		SkipFinalSnapshot:   pulumi.Bool(debug),
	})

	if err != nil {
		return err
	}
	ctx.Export("dbPasswordParameter", ssmParameter.Name)
	ctx.Export("dbEndpoint", db.Endpoint)

	return nil
}
