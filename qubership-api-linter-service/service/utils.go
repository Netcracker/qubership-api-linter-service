package service

import (
	"context"
	"fmt"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/utils"
)

func getVersionAndRevision(ctx context.Context, cl client.ApihubClient, packageId string, version string) (string, int, error) {
	ver, rev, err := utils.SplitVersionRevision(version)
	if err != nil {
		return "", 0, err
	}

	if rev == 0 {
		versionView, err := cl.GetVersion(ctx, packageId, version)
		if err != nil {
			return "", 0, err
		}
		if versionView == nil {
			return "", 0, fmt.Errorf("version %s not found in package %s", version, packageId)
		}
		ver, rev, err = utils.SplitVersionRevision(versionView.Version)
		if err != nil {
			return "", 0, err
		}
		if rev == 0 {
			return "", 0, fmt.Errorf("unable to identify latest revision for version %s", version)
		}
	}
	return ver, rev, nil
}
