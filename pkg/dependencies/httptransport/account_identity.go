package httptransport

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// accountIdentity only accepts an unambiguous identity from the authenticated
// response. Missing metadata remains compatible with discovery-only callers.
func accountIdentity(headers http.Header, operation string) (*sdm.AccountIdentity, error) {
	var values []string

	for key, entries := range headers {
		if strings.EqualFold(key, protocol.HeaderUserID) {
			values = append(values, entries...)
		}
	}

	if len(values) == 0 {
		//nolint:nilnil // Absent optional response metadata is distinct from a malformed identity.
		return nil, nil
	}

	if len(values) != 1 || values[0] == "" || !utf8.ValidString(values[0]) ||
		strings.ContainsFunc(values[0], invalidIdentityCharacter) {
		return nil, fail(operation, ErrorInvalidResponse, nil)
	}

	return &sdm.AccountIdentity{UserId: values[0]}, nil
}

func invalidIdentityCharacter(value rune) bool {
	return value <= ' ' || value == '\x7f' || value == ','
}
