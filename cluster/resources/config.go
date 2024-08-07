package resources

import (
	"encoding/base64"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func safeName(name string) string {
	return base64.URLEncoding.EncodeToString([]byte(name))
}

func getFirstSubnetID(cfg *config.Config, public bool) string {
	return getSubnetIDs(cfg, public)[0]
}

func getSubnetIDs(cfg *config.Config, public bool) []string {
	var subnets []string
	var key string
	if public {
		key = "vpcPublicSubnetARNs"

	} else {
		key = "vpcPrivateSubnetARNs"
	}
	cfg.RequireObject(key, &subnets)
	return subnets
}

func getSSHCIDRs(cfg *config.Config) []string {
	var cidrs []string
	cfg.RequireObject("sshCIDRs", &cidrs)
	return cidrs
}