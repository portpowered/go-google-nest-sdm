package httptransport

import (
	"context"
	"net/url"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

// OAuthExchange exchanges an authorization code and returns credentials to the caller.
// It does not retain credentials or implicitly complete the required initial devices.list call.
func (client *Client) OAuthExchange(
	ctx context.Context, input wire.OAuthTokenRequest,
) (wire.OAuthTokenResponse, error) {
	return client.oauth(ctx, "OAuthExchange", input, protocol.OAuthGrantAuthorizationCode)
}

// OAuthRefresh explicitly renews caller-owned credentials without mutating the client.
func (client *Client) OAuthRefresh(ctx context.Context, input wire.OAuthTokenRequest) (wire.OAuthTokenResponse, error) {
	return client.oauth(ctx, "OAuthRefresh", input, protocol.OAuthGrantRefreshToken)
}

func (client *Client) oauth(
	ctx context.Context, operation string, input wire.OAuthTokenRequest, grant string,
) (wire.OAuthTokenResponse, error) {
	var result wire.OAuthTokenResponse
	if input.ClientId == "" || input.ClientSecret == "" {
		return result, fail(operation, ErrorInvalidRequest, nil)
	}

	values := url.Values{}
	values.Set(protocol.FormClientID, input.ClientId)
	values.Set(protocol.FormClientSecret, input.ClientSecret)
	values.Set(protocol.FormGrantType, grant)

	if input.CodeVerifier != nil && *input.CodeVerifier == "" {
		return result, fail(operation, ErrorInvalidRequest, nil)
	}

	if grant == protocol.OAuthGrantAuthorizationCode {
		if input.Code == nil || *input.Code == "" || input.RedirectUri == nil || *input.RedirectUri == "" {
			return result, fail(operation, ErrorInvalidRequest, nil)
		}

		values.Set(protocol.FormCode, *input.Code)
		values.Set(protocol.FormRedirectURI, *input.RedirectUri)

		if input.CodeVerifier != nil {
			values.Set(protocol.FormCodeVerifier, *input.CodeVerifier)
		}
	} else {
		if input.RefreshToken == nil || *input.RefreshToken == "" {
			return result, fail(operation, ErrorInvalidRequest, nil)
		}

		values.Set(protocol.FormRefreshToken, *input.RefreshToken)
	}

	err := client.exchange(
		ctx, operation, protocol.MethodOAuthToken, client.oauthBaseURL+protocol.PathOAuthToken,
		"", protocol.MIMEFormURLEncoded, strings.NewReader(values.Encode()), &result,
	)
	if err == nil && (result.AccessToken == "" || result.ExpiresIn <= 0 || result.TokenType == "") {
		return result, fail(operation, ErrorInvalidResponse, nil)
	}

	return result, err
}
