package resources

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func GetALBSANs(cfg *config.Config) []string {
	var keys []string
	cfg.RequireObject("albSANs", &keys)
	if len(keys) == 0 {
		return []string{}
	}
	return keys[1:]
}
