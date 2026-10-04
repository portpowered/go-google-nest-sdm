package httptransport

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

var errPushData = errors.New("missing event data")

func (*sdkClient) DecodePushEvent(
	ctx context.Context, request sdm.DecodePushEventRequest,
) (sdm.DecodePushEventResult, error) {
	if ctx == nil {
		return sdm.DecodePushEventResult{}, publicError(fail("DecodePushEvent", ErrorInvalidRequest, nil))
	}

	err := ctx.Err()
	if err != nil {
		return sdm.DecodePushEventResult{}, sessionContextError("DecodePushEvent", err)
	}

	data := request.Body

	if !request.Unwrapped {
		var envelope wire.PushEnvelope

		err := json.Unmarshal(data, &envelope)
		if err != nil {
			return sdm.DecodePushEventResult{}, publicError(fail("DecodePushEvent", ErrorInvalidResponse, err))
		}

		_, err = resourcePath(envelope.Subscription, protocol.PathPull,
			protocol.CollectionProjects, protocol.CollectionSubscriptions)
		if err != nil {
			return sdm.DecodePushEventResult{}, publicError(fail("DecodePushEvent", ErrorInvalidResponse, err))
		}

		if envelope.Message.Data == nil {
			return sdm.DecodePushEventResult{}, publicError(fail("DecodePushEvent", ErrorInvalidResponse, errPushData))
		}

		data = *envelope.Message.Data
	}

	event, err := sdm.DecodeEvent(data)
	if err != nil {
		return sdm.DecodePushEventResult{}, publicError(fail("DecodePushEvent", ErrorInvalidResponse, err))
	}

	return sdm.DecodePushEventResult{Event: event}, nil
}
