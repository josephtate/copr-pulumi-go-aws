package resources

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func getDefaultTags(cfg *config.Config) map[string]string {
	var ret map[string]string
	cfg.RequireObject("defaultTags", &ret)
	return ret
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

func getAdminSSHKeys(cfg *config.Config) []string {
	var keys []string
	cfg.RequireObject("sshAdminKeys", &keys)
	return keys
}
