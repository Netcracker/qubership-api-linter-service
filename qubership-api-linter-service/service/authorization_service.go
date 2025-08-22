package service

import (
	"context"
	"github.com/Netcracker/qubership-api-linter-service/client"
	"github.com/Netcracker/qubership-api-linter-service/secctx"
)

type AuthorizationService interface {
	HasRulesetReadPermission(ctx context.Context) (bool, error)
	HasRulesetManagementPermission(ctx context.Context) (bool, error)

	HasReadPackagePermission(ctx context.Context, packageId string) (bool, error)
	HasPublishPackagePermission(ctx context.Context, packageId string) (bool, error)
}

func NewAuthorizationService(apihubClient client.ApihubClient) AuthorizationService {
	return &authorizationServiceImpl{apihubClient: apihubClient}
}

type authorizationServiceImpl struct {
	apihubClient client.ApihubClient
}

func (a authorizationServiceImpl) HasRulesetReadPermission(ctx context.Context) (bool, error) {
	return true, nil // No restrictions at this moment
}

func (a authorizationServiceImpl) HasRulesetManagementPermission(ctx context.Context) (bool, error) {
	return secctx.IsSysadm(ctx), nil
}

func (a authorizationServiceImpl) HasReadPackagePermission(ctx context.Context, packageId string) (bool, error) {
	// TODO: Authorization check:
	// TODO: make sure that user have a access to the version
	// TODO: send special request to Apihub to check it!
	// TODO: smt like remote HasRequiredPermissions(ctx, packageId, view.ReadPermission)

	//TODO implement me
	return true, nil // FIXME: just for testing!
}

func (a authorizationServiceImpl) HasPublishPackagePermission(ctx context.Context, packageId string) (bool, error) {
	//TODO implement me
	return true, nil
}
