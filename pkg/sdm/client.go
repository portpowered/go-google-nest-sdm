// Package sdm provides transport-independent Smart Device Management models and operations.
package sdm

import "context"

// Client serves multiple accounts using the credentials supplied with each request.
// Commands acknowledge acceptance; they do not prove the physical action completed.
// The client never refreshes credentials or retries commands implicitly.
//
//nolint:interfacebloat // API-01 groups the complete provider operation surface for tracing.
type Client interface {
	AuthClient
	EventClient
	ListDevices(ctx context.Context, request ListDevicesRequest) (ListDevicesResult, error)
	GetDevice(ctx context.Context, request GetDeviceRequest) (GetDeviceResult, error)
	ListStructures(ctx context.Context, request ListStructuresRequest) (ListStructuresResult, error)
	GetStructure(ctx context.Context, request GetStructureRequest) (GetStructureResult, error)
	ListRooms(ctx context.Context, request ListRoomsRequest) (ListRoomsResult, error)
	GetRoom(ctx context.Context, request GetRoomRequest) (GetRoomResult, error)
	SetFanTimer(ctx context.Context, request SetFanTimerRequest) (SetFanTimerResult, error)
	SetThermostatEcoMode(ctx context.Context, request SetThermostatEcoModeRequest) (SetThermostatEcoModeResult, error)
	SetThermostatMode(ctx context.Context, request SetThermostatModeRequest) (SetThermostatModeResult, error)
	SetHeat(ctx context.Context, request SetHeatRequest) (SetHeatResult, error)
	SetCool(ctx context.Context, request SetCoolRequest) (SetCoolResult, error)
	SetRange(ctx context.Context, request SetRangeRequest) (SetRangeResult, error)
	GenerateImage(ctx context.Context, request GenerateImageRequest) (GenerateImageResult, error)
	GenerateRtspStream(ctx context.Context, request GenerateRtspStreamRequest) (GenerateRtspStreamResult, error)
	ExtendRtspStream(ctx context.Context, request ExtendRtspStreamRequest) (ExtendRtspStreamResult, error)
	StopRtspStream(ctx context.Context, request StopRtspStreamRequest) (StopRtspStreamResult, error)
	GenerateWebRtcStream(ctx context.Context, request GenerateWebRtcStreamRequest) (GenerateWebRtcStreamResult, error)
	ExtendWebRtcStream(ctx context.Context, request ExtendWebRtcStreamRequest) (ExtendWebRtcStreamResult, error)
	StopWebRtcStream(ctx context.Context, request StopWebRtcStreamRequest) (StopWebRtcStreamResult, error)
}

// AuthClient explicitly returns OAuth credentials for callers to store and renew.
type AuthClient interface {
	ExchangeToken(ctx context.Context, request ExchangeTokenRequest) (ExchangeTokenResult, error)
	RefreshToken(ctx context.Context, request RefreshTokenRequest) (RefreshTokenResult, error)
}

// AuthorizationClient opens an explicit account-linking session. Existing Client
// implementations remain compatible; the HTTP client also implements this interface.
type AuthorizationClient interface {
	OpenAuthorizationSession(ctx context.Context, request OpenAuthorizationSessionRequest) (AuthorizationSession, error)
}

// AccountAuthorizationClient supplies token exchange and the mandatory initial
// device discovery without retaining account credentials on a reusable client.
type AccountAuthorizationClient interface {
	ExchangeToken(ctx context.Context, request ExchangeTokenRequest) (ExchangeTokenResult, error)
	ListDevices(ctx context.Context, request ListDevicesRequest) (ListDevicesResult, error)
}

// AuthorizationSession binds OAuth state, PKCE, and the exact callback URI to one
// account-linking attempt. The caller owns its browser and HTTP endpoint, closes
// the session, and explicitly stores credentials returned by Complete.
type AuthorizationSession interface {
	// AuthorizationURL is the PCM browser destination for this attempt.
	AuthorizationURL() string
	// Complete validates the callback, exchanges its code, and performs initial
	// discovery. ErrorInvalidRequest with Operation CompleteAuthorization allows
	// another callback attempt. Other failures consume or close the attempt;
	// token/discovery failures report Operation AuthorizeAccount.
	Complete(ctx context.Context, request CompleteAuthorizationRequest) (CompleteAuthorizationResult, error)
	// Close cancels completion and erases retained secrets; repeated calls are safe.
	Close() error
}

// EventClient opens an account-bound Pub/Sub pull session.
type EventClient interface {
	OpenEventSession(ctx context.Context, request OpenEventSessionRequest) (EventSession, error)
	// DecodePushEvent decodes data only. The HTTP handler verifies push identity
	// and audience, then acknowledges processing using its HTTP response status.
	DecodePushEvent(ctx context.Context, request DecodePushEventRequest) (DecodePushEventResult, error)
}

// EventSession owns its pull lifecycle. The caller closes it; repeated Close is safe.
// Delivery acknowledgements are explicit so decoding or handling failures can be retried.
type EventSession interface {
	Next(ctx context.Context) (EventDelivery, error)
	Close() error
}

// EventDelivery retains an SDM event and the Pub/Sub acknowledgement controls.
// Acknowledge only after successfully processing the event.
type EventDelivery interface {
	Event() EventEnvelope
	Acknowledge(ctx context.Context) error
	ModifyAckDeadline(ctx context.Context, request AckDeadlineRequest) error
}

// MediaClient retrieves images and clip previews from returned media URLs.
// The caller owns and closes each result body.
type MediaClient interface {
	DownloadImage(ctx context.Context, request DownloadImageRequest) (DownloadImageResult, error)
	DownloadClipPreview(ctx context.Context, request DownloadClipPreviewRequest) (DownloadClipPreviewResult, error)
}
