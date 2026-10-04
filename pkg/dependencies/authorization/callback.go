package authorization

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func validRequest(request sdm.OpenAuthorizationSessionRequest) bool {
	if request.ClientId == "" || request.ClientSecret == "" || request.ProjectId == "" {
		return false
	}

	for _, char := range request.ProjectId {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '_' {
			return false
		}
	}

	redirect, err := url.Parse(request.RedirectUri)
	if err != nil || redirect.Host == "" || redirect.User != nil || redirect.RawQuery != "" ||
		redirect.ForceQuery || redirect.Fragment != "" {
		return false
	}

	if redirect.Scheme == "https" {
		return true
	}

	ip := net.ParseIP(redirect.Hostname())

	return redirect.Scheme == "http" && (redirect.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}

func consentURL(request sdm.OpenAuthorizationSessionRequest, state, verifier string) string {
	challenge := sha256.Sum256([]byte(verifier))
	values := url.Values{}
	values.Set(protocol.QueryParamClientID, request.ClientId)
	values.Set(protocol.QueryParamRedirectURI, request.RedirectUri)
	values.Set(protocol.QueryParamResponseType, protocol.OAuthResponseTypeCode)
	values.Set(protocol.QueryParamScope, protocol.OAuthScopeSDM)
	values.Set(protocol.QueryParamAccessType, protocol.OAuthAccessTypeOffline)
	values.Set(protocol.QueryParamPrompt, protocol.OAuthPromptConsent)
	values.Set(protocol.QueryParamState, state)
	values.Set(protocol.QueryParamCodeChallenge, base64.RawURLEncoding.EncodeToString(challenge[:]))
	values.Set(protocol.QueryParamCodeChallengeMethod, protocol.OAuthPKCES256)

	return protocol.PCMBaseURL + fmt.Sprintf(protocol.PathPCMConsent, request.ProjectId) + "?" + values.Encode()
}

func callbackCode(callbackURL, redirectURI, state string) (string, error) {
	callback, err := url.Parse(callbackURL)

	redirect, redirectErr := url.Parse(redirectURI)

	if err != nil || redirectErr != nil || callback.Scheme != redirect.Scheme || callback.Host != redirect.Host ||
		callback.EscapedPath() != redirect.EscapedPath() || callback.User != nil || callback.Fragment != "" {
		return "", failure("CompleteAuthorization", sdm.ErrorInvalidRequest, nil)
	}

	values, err := url.ParseQuery(callback.RawQuery)
	if err != nil {
		return "", failure("CompleteAuthorization", sdm.ErrorInvalidRequest, err)
	}

	keys := []string{protocol.QueryParamState, protocol.QueryParamCode,
		protocol.QueryParamError, protocol.QueryParamErrorDescription}
	for _, key := range keys {
		if len(values[key]) > 1 {
			return "", failure("CompleteAuthorization", sdm.ErrorInvalidRequest, nil)
		}
	}

	if subtle.ConstantTimeCompare([]byte(values.Get(protocol.QueryParamState)), []byte(state)) != 1 {
		return "", failure("CompleteAuthorization", sdm.ErrorInvalidRequest, nil)
	}

	code, denied := values.Get(protocol.QueryParamCode), values.Get(protocol.QueryParamError)
	if strings.TrimSpace(code) == "" && denied == "" || code != "" && denied != "" {
		return "", failure("CompleteAuthorization", sdm.ErrorInvalidRequest, nil)
	}

	if denied != "" {
		return "", failure("CompleteAuthorization", sdm.ErrorUnauthorized, nil)
	}

	return code, nil
}
