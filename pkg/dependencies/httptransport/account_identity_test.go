package httptransport_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const identityHeader = "Userid"
const opaqueAccount = "opaque-account"
const missingIdentity = "missing"

func TestAuthenticatedAccountIdentity(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name     string
		headers  http.Header
		identity string
		invalid  bool
	}{
		{name: missingIdentity, headers: nil, identity: "", invalid: false},
		{name: "canonical", headers: http.Header{identityHeader: {opaqueAccount}}, identity: opaqueAccount, invalid: false},
		{name: "case insensitive", headers: http.Header{"USERID": {opaqueAccount}}, identity: opaqueAccount, invalid: false},
		{name: "unrelated", headers: http.Header{"Other": {"ignored"}}, identity: "", invalid: false},
		{name: "empty", headers: http.Header{identityHeader: {""}}, identity: "", invalid: true},
		{name: "duplicate", headers: http.Header{identityHeader: {"a", "a"}}, identity: "", invalid: true},
		{name: "duplicate casing", headers: http.Header{identityHeader: {"a"}, "userid": {"b"}}, identity: "", invalid: true},
		{name: "combined", headers: http.Header{identityHeader: {"a,b"}}, identity: "", invalid: true},
		{name: "whitespace", headers: http.Header{identityHeader: {"a b"}}, identity: "", invalid: true},
		{name: "form feed", headers: http.Header{identityHeader: {"a\fb"}}, identity: "", invalid: true},
		{name: "control", headers: http.Header{identityHeader: {"a\x00b"}}, identity: "", invalid: true},
		{name: "invalid UTF-8", headers: http.Header{identityHeader: {"a\xffb"}}, identity: "", invalid: true},
		{name: "delete", headers: http.Header{identityHeader: {"a\x7fb"}}, identity: "", invalid: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			doer := sdkDoer(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet || request.URL.Path != "/v1/enterprises/e/devices" ||
					request.Header.Get("Authorization") != "Bearer account" {
					t.Errorf("unexpected device-list request")
				}

				response := sdkResponse(http.StatusOK, `{"devices":[]}`)
				response.Header = testCase.headers

				return response, nil
			})

			client, err := transport.NewClient(transport.WithHTTPClient(doer))
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.ListDevices(context.Background(), sdm.ListDevicesRequest{
				Auth: sdm.AuthContext{AccessToken: "account"}, Parent: boundaryParent, Filter: nil,
			})
			if testCase.invalid {
				if err == nil || result.AccountIdentity != nil {
					t.Fatal("accepted ambiguous identity")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if testCase.identity == "" {
				if result.AccountIdentity != nil {
					t.Fatal("invented account identity")
				}
			} else if result.AccountIdentity == nil || result.AccountIdentity.UserId != testCase.identity {
				t.Fatal("lost authenticated account identity")
			}
		})
	}
}

func TestSharedClientIdentityIsolation(t *testing.T) {
	t.Parallel()

	doer := sdkDoer(func(request *http.Request) (*http.Response, error) {
		account := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		response := sdkResponse(http.StatusOK, `{"devices":[]}`)
		response.Header = http.Header{identityHeader: {account}}

		switch account {
		case missingIdentity:
			response.Header = nil
		case "unauthorized":
			response.StatusCode = http.StatusUnauthorized
		case "malformed":
			response = sdkResponse(http.StatusOK, `{"devices":null}`)
			response.Header = http.Header{identityHeader: {account}}
		}

		return response, nil
	})

	client, err := transport.NewClient(transport.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}

	// A successful call precedes missing and failed calls on the same client.
	for _, account := range []string{"first", missingIdentity, "unauthorized", "malformed"} {
		result, err := client.ListDevices(context.Background(), sdm.ListDevicesRequest{
			Auth: sdm.AuthContext{AccessToken: account}, Parent: boundaryParent, Filter: nil,
		})
		if account == "first" {
			if err != nil || result.AccountIdentity == nil || result.AccountIdentity.UserId != account {
				t.Fatal("successful call lost identity")
			}
		} else if result.AccountIdentity != nil {
			t.Fatal("later call retained earlier identity")
		}
	}

	for _, account := range []string{"account-A", "account-B"} {
		t.Run(account, func(t *testing.T) {
			t.Parallel()

			result, err := client.ListDevices(context.Background(), sdm.ListDevicesRequest{
				Auth: sdm.AuthContext{AccessToken: account}, Parent: boundaryParent, Filter: nil,
			})
			if err != nil || result.AccountIdentity == nil || result.AccountIdentity.UserId != account {
				t.Fatal("shared client crossed account identities")
			}
		})
	}
}
