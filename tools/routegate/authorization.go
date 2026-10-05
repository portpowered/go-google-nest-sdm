package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
)

// The browser performs this GET rather than the SDK's HTTP transport. This
// inventoried constructor binds the authority, path, query keys and PKCE values
// before the URL is handed to the caller. No arbitrary URL transformations are
// approved by the browser boundary.
const consentConstructor = `package authorization
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
}`

func auditAuthorization(root string) error {
	file, err := parser.ParseFile(token.NewFileSet(),
		filepath.Join(root, "pkg/dependencies/authorization/callback.go"), nil, 0)
	if err != nil {
		return fmt.Errorf("parse consent constructor: %w", err)
	}

	return verifyConsentConstructor(file)
}

func verifyConsentConstructor(file *ast.File) error {
	imports, err := auditImports(file)
	if err != nil {
		return err
	}

	for alias, owner := range map[string]string{"sha256": "crypto/sha256", "base64": "encoding/base64",
		"url": urlImport, "fmt": "fmt", "protocol": module + "/internal/protocol", "sdm": module + "/pkg/sdm"} {
		if imports[alias] != owner {
			return fmt.Errorf("%w: consent constructor import %s is shadowed", errRouteInvalid, alias)
		}
	}

	expected, err := parser.ParseFile(token.NewFileSet(), "consent.go", consentConstructor, 0)
	if err != nil {
		return fmt.Errorf("parse consent inventory: %w", err)
	}

	for _, declaration := range file.Decls {
		function, recognized := declaration.(*ast.FuncDecl)
		if recognized && function.Name.Name == "consentURL" {
			var actual, owned bytes.Buffer

			actualErr := format.Node(&actual, token.NewFileSet(), function)

			ownedErr := format.Node(&owned, token.NewFileSet(), expected.Decls[0])

			if actualErr == nil && ownedErr == nil && bytes.Equal(actual.Bytes(), owned.Bytes()) {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: consent URL differs from inventoried generated route/query/PKCE constructor", errRouteInvalid)
}
