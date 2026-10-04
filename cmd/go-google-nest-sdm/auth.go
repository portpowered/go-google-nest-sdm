package main

import (
	"context"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type authSummary struct {
	ExpiresIn       int    `json:"expiresIn"`
	TokenType       string `json:"tokenType"`
	HasRefreshToken bool   `json:"hasRefreshToken"`
}

func (app application) authenticate(
	ctx context.Context, operation string, options invocation, account credentials,
) (any, error) {
	if operation == "export" {
		return account, nil
	}

	client, ok := app.client.(sdm.AuthClient)
	if !ok {
		return nil, errAuthenticationUnsupported
	}

	var value sdm.Credentials

	switch operation {
	case "exchange":
		request := sdm.ExchangeTokenRequest{
			ClientId: account.ClientID, ClientSecret: account.ClientSecret,
			Code: account.AuthorizationCode, RedirectUri: account.RedirectURI, CodeVerifier: nil,
		}
		if account.CodeVerifier != "" {
			request.CodeVerifier = &account.CodeVerifier
		}

		result, err := client.ExchangeToken(ctx, request)
		if err != nil {
			return nil, wrapError(err)
		}

		value = result.Credentials
	case "refresh":
		result, err := client.RefreshToken(ctx, sdm.RefreshTokenRequest{
			ClientId: account.ClientID, ClientSecret: account.ClientSecret,
			RefreshToken: account.RefreshToken,
		})
		if err != nil {
			return nil, wrapError(err)
		}

		value = result.Credentials
	default:
		return nil, errAuthOperation
	}

	if options.export {
		account.AccessToken = value.AccessToken
		if value.RefreshToken != nil {
			account.RefreshToken = *value.RefreshToken
		}

		account.AuthorizationCode = ""
		account.CodeVerifier = ""

		return account, nil
	}

	return authSummary{
		ExpiresIn: value.ExpiresIn, TokenType: value.TokenType,
		HasRefreshToken: value.RefreshToken != nil || account.RefreshToken != "",
	}, nil
}
