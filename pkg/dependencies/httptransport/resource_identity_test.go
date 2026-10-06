package httptransport_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	transport "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

const (
	resourceIdentityValid        = "resource identity valid"
	resourceIdentityMalformed    = "resource identity malformed"
	resourceIdentityInvalidBody  = "resource identity invalid body"
	resourceIdentityUnauthorized = "resource identity unauthorized"
)

// Synthetic responses exercise optional authenticated metadata on both routes.
func TestResourceAccountIdentity(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"GetDevice", "ListStructures"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			for _, scenario := range []string{
				resourceIdentityValid, missingIdentity, resourceIdentityMalformed,
				resourceIdentityInvalidBody, resourceIdentityUnauthorized,
			} {
				t.Run(scenario, func(t *testing.T) {
					t.Parallel()

					client, err := transport.NewClient(transport.WithHTTPClient(resourceIdentityDoer(t, operation, scenario)))
					if err != nil {
						t.Fatal(err)
					}

					identity, err := callResourceIdentity(client, operation)
					assertResourceIdentity(t, operation, scenario, identity, err)
				})
			}
		})
	}
}

func resourceIdentityDoer(t *testing.T, operation, scenario string) sdkDoer {
	t.Helper()

	return func(request *http.Request) (*http.Response, error) {
		path, body := "/v1/enterprises/e/devices/d", `{"name":"enterprises/e/devices/d"}`
		if operation == "ListStructures" {
			path, body = "/v1/enterprises/e/structures", `{"structures":[]}`
		}

		if request.Method != http.MethodGet || request.URL.Path != path ||
			request.Header.Get("Authorization") != "Bearer "+testAccountToken {
			t.Error("unexpected authenticated resource request")
		}

		response := sdkResponse(http.StatusOK, body)
		response.Header = http.Header{identityHeader: {opaqueAccount}}

		switch scenario {
		case missingIdentity:
			response.Header = nil
		case resourceIdentityMalformed:
			response.Header = http.Header{identityHeader: {"a", "b"}}
		case resourceIdentityInvalidBody:
			response = sdkResponse(http.StatusOK, `null`)
			response.Header = http.Header{identityHeader: {opaqueAccount}}
		case resourceIdentityUnauthorized:
			response.StatusCode = http.StatusUnauthorized
		}

		return response, nil
	}
}

func callResourceIdentity(client sdm.Client, operation string) (*sdm.AccountIdentity, error) {
	auth := sdm.AuthContext{AccessToken: testAccountToken}
	if operation == "GetDevice" {
		result, err := client.GetDevice(context.Background(), sdm.GetDeviceRequest{Auth: auth, Name: testShortDeviceName})
		if err != nil {
			return nil, fmt.Errorf("resource identity call: %w", err)
		}

		return result.AccountIdentity, nil
	}

	result, err := client.ListStructures(
		context.Background(), sdm.ListStructuresRequest{Auth: auth, Parent: boundaryParent},
	)
	if err != nil {
		return nil, fmt.Errorf("resource identity call: %w", err)
	}

	return result.AccountIdentity, nil
}

func assertResourceIdentity(t *testing.T, operation, scenario string, identity *sdm.AccountIdentity, err error) {
	t.Helper()

	switch scenario {
	case resourceIdentityValid:
		if err != nil || identity == nil || identity.UserId != opaqueAccount {
			t.Fatal("lost resource account identity")
		}
	case missingIdentity:
		if err != nil || identity != nil {
			t.Fatal("invented resource account identity")
		}
	default:
		var failure *sdm.Error
		if !errors.As(err, &failure) || failure.Operation != operation || identity != nil {
			t.Fatal("accepted failed resource identity")
		}
	}
}
