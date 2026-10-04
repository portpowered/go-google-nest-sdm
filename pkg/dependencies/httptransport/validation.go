package httptransport

import (
	"fmt"

	"github.com/portpowered/go-google-nest-sdm/internal/contracts"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

// Validation uses the original bytes so explicit null and missing fields cannot
// disappear through generated pointer decoding. Unknown schema-approved fields remain open.
func validateDependencyResponse(payload []byte, result any) error {
	var document, component string

	switch result.(type) {
	case *wire.OAuthTokenResponse:
		document, component = "oauth.openapi.yaml", "OAuthTokenResponse"
	case *wire.PullResponse:
		document, component = "pubsub.openapi.yaml", "PullResponse"
	case *wire.EmptyResponse:
		document, component = "pubsub.openapi.yaml", "EmptyResponse"
	default:
		return nil
	}

	err := contracts.Validate(document, component, payload)
	if err != nil {
		return fmt.Errorf("validate dependency response: %w", err)
	}

	return nil
}
