package httptransport

import (
	"context"

	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func (client *sdkClient) ExchangeToken(
	ctx context.Context,
	request sdm.ExchangeTokenRequest,
) (sdm.ExchangeTokenResult, error) {
	response,
		err := client.transport.OAuthExchange(
		ctx,
		wire.OAuthTokenRequest{
			ClientId:     request.ClientId,
			ClientSecret: request.ClientSecret,
			Code:         &request.Code,
			RedirectUri:  &request.RedirectUri,
			CodeVerifier: request.CodeVerifier,
			GrantType:    wire.AuthorizationCode,
			RefreshToken: nil,
		},
	)
	if err != nil {
		return sdm.ExchangeTokenResult{}, publicError(err)
	}

	return sdm.ExchangeTokenResult{Credentials: credentials(response)}, nil
}

func (client *sdkClient) RefreshToken(
	ctx context.Context,
	request sdm.RefreshTokenRequest,
) (sdm.RefreshTokenResult, error) {
	response,
		err := client.transport.OAuthRefresh(
		ctx,
		wire.OAuthTokenRequest{
			ClientId:     request.ClientId,
			ClientSecret: request.ClientSecret,
			RefreshToken: &request.RefreshToken,
			Code:         nil,
			CodeVerifier: nil,
			RedirectUri:  nil,
			GrantType:    wire.RefreshToken,
		},
	)
	if err != nil {
		return sdm.RefreshTokenResult{}, publicError(err)
	}

	return sdm.RefreshTokenResult{Credentials: credentials(response)}, nil
}

func credentials(response wire.OAuthTokenResponse) sdm.Credentials {
	return sdm.Credentials{
		AccessToken:           response.AccessToken,
		TokenType:             response.TokenType,
		ExpiresIn:             response.ExpiresIn,
		RefreshToken:          response.RefreshToken,
		RefreshTokenExpiresIn: response.RefreshTokenExpiresIn,
		Scope:                 response.Scope,
	}
}
