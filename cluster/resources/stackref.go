package resources

import (
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type stackRefResult struct {
	ref *pulumi.StackReference
	err error
}

var (
	vpcStackRefs     = make(map[string]stackRefResult)
	vpcStackRefsLock sync.Mutex
)

func GetStackRef(ctx *pulumi.Context, project string) (*pulumi.StackReference, error) {

	callStack := ctx.Stack()
	parts := strings.Split(callStack, "/")
	stackName := parts[len(parts)-1]

	vpcStackRefsLock.Lock()
	defer vpcStackRefsLock.Unlock()

	if result, exists := vpcStackRefs[project]; exists {
		return result.ref, result.err
	}

	ref, err := pulumi.NewStackReference(ctx, project+"/"+stackName, nil)
	result := stackRefResult{ref: ref, err: err}

	vpcStackRefs[project] = result

	return ref, err
}

func getReferenceValue(ctx *pulumi.Context, project string, key string) (pulumi.StringOutput, error) {
	stackRef, err := GetStackRef(ctx, project)

	if err != nil {
		return pulumi.String("").ToStringOutput(), err
	}

	value := stackRef.GetStringOutput(pulumi.String(key))

	return value, nil
}

func GetVPCID(ctx *pulumi.Context, project string) (pulumi.StringOutput, error) {
	return getReferenceValue(ctx, project, "vpcId")
}

func GetPublicHostedZoneID(ctx *pulumi.Context, project string) (pulumi.StringOutput, error) {
	return getReferenceValue(ctx, project, "publicHostedZoneId")
}

func GetPrivateHostedZoneID(ctx *pulumi.Context, project string) (pulumi.StringOutput, error) {
	return getReferenceValue(ctx, project, "privateHostedZoneId")
}

func GetPublicDomainName(ctx *pulumi.Context, project string) (pulumi.StringOutput, error) {
	return getReferenceValue(ctx, project, "publicDomainName")
}

func GetPrivateDomainName(ctx *pulumi.Context, project string) (pulumi.StringOutput, error) {
	return getReferenceValue(ctx, project, "privateDomainName")
}

func GetSubnets(ctx *pulumi.Context, project string, public bool) (pulumi.StringArrayOutput, error) {
	stackRef, err := GetStackRef(ctx, project)

	if err != nil {
		return pulumi.StringArrayOutput{}, err
	}
	key := "publicSubnets"
	if !public {
		key = "privateSubnets"
	}

	value := stackRef.GetOutput(pulumi.String(key)).AsStringArrayOutput()

	return value, nil
}

func GetFirstSubnet(ctx *pulumi.Context, project string, public bool) (pulumi.StringOutput, error) {
	subnets, err := GetSubnets(ctx, project, public)

	if err != nil {
		return pulumi.String("").ToStringOutput(), err
	}

	return subnets.Index(pulumi.Int(0)), nil
}
