// Package sdm provides transport-independent Smart Device Management models and operations.
package sdm

import "context"

// Client serves multiple accounts using the credentials supplied with each request.
// Commands acknowledge acceptance; they do not prove the physical action completed.
// The client never refreshes credentials or retries commands implicitly.
type Client interface {
	AuthClient
	EventClient
	ListDevices(context.Context, ListDevicesRequest) (ListDevicesResult, error)
	GetDevice(context.Context, GetDeviceRequest) (GetDeviceResult, error)
	ListStructures(context.Context, ListStructuresRequest) (ListStructuresResult, error)
	GetStructure(context.Context, GetStructureRequest) (GetStructureResult, error)
	ListRooms(context.Context, ListRoomsRequest) (ListRoomsResult, error)
	GetRoom(context.Context, GetRoomRequest) (GetRoomResult, error)
	SetFanTimer(context.Context, SetFanTimerRequest) (SetFanTimerResult, error)
	SetThermostatEcoMode(context.Context, SetThermostatEcoModeRequest) (SetThermostatEcoModeResult, error)
	SetThermostatMode(context.Context, SetThermostatModeRequest) (SetThermostatModeResult, error)
	SetHeat(context.Context, SetHeatRequest) (SetHeatResult, error)
	SetCool(context.Context, SetCoolRequest) (SetCoolResult, error)
	SetRange(context.Context, SetRangeRequest) (SetRangeResult, error)
	GenerateImage(context.Context, GenerateImageRequest) (GenerateImageResult, error)
	GenerateRtspStream(context.Context, GenerateRtspStreamRequest) (GenerateRtspStreamResult, error)
	ExtendRtspStream(context.Context, ExtendRtspStreamRequest) (ExtendRtspStreamResult, error)
	StopRtspStream(context.Context, StopRtspStreamRequest) (StopRtspStreamResult, error)
	GenerateWebRtcStream(context.Context, GenerateWebRtcStreamRequest) (GenerateWebRtcStreamResult, error)
	ExtendWebRtcStream(context.Context, ExtendWebRtcStreamRequest) (ExtendWebRtcStreamResult, error)
	StopWebRtcStream(context.Context, StopWebRtcStreamRequest) (StopWebRtcStreamResult, error)
}

// AuthClient explicitly returns OAuth credentials for callers to store and renew.
type AuthClient interface {
	ExchangeToken(context.Context, ExchangeTokenRequest) (ExchangeTokenResult, error)
	RefreshToken(context.Context, RefreshTokenRequest) (RefreshTokenResult, error)
}

// EventClient opens an account-bound Pub/Sub pull session.
type EventClient interface {
	OpenEventSession(context.Context, OpenEventSessionRequest) (EventSession, error)
	// DecodePushEvent decodes data only. The HTTP handler verifies push identity
	// and audience, then acknowledges processing using its HTTP response status.
	DecodePushEvent(context.Context, DecodePushEventRequest) (DecodePushEventResult, error)
}

// EventSession owns its pull lifecycle. The caller closes it; repeated Close is safe.
// Delivery acknowledgements are explicit so decoding or handling failures can be retried.
type EventSession interface {
	Next(context.Context) (EventDelivery, error)
	Close() error
}

// EventDelivery retains an SDM event and the Pub/Sub acknowledgement controls.
// Acknowledge only after successfully processing the event.
type EventDelivery interface {
	Event() EventEnvelope
	Acknowledge(context.Context) error
	ModifyAckDeadline(context.Context, AckDeadlineRequest) error
}

// MediaClient retrieves images and clip previews from returned media URLs.
// The caller owns and closes each result body.
type MediaClient interface {
	DownloadImage(context.Context, DownloadImageRequest) (DownloadImageResult, error)
	DownloadClipPreview(context.Context, DownloadClipPreviewRequest) (DownloadClipPreviewResult, error)
}
