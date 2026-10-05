package httptransport

import (
	"context"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/authorization"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// OpenAuthorizationSession creates a caller-owned account-linking attempt.
// It does not start a callback server, open a browser, or change client credentials.
//
//nolint:ireturn // API-13 presents account-linking state through the public session interface.
func (client *sdkClient) OpenAuthorizationSession(
	ctx context.Context, request sdm.OpenAuthorizationSessionRequest,
) (sdm.AuthorizationSession, error) {
	session, err := authorization.NewSession(ctx, client, request)

	return session, publicError(err)
}
