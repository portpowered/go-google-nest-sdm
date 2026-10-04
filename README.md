# go-google-nest-sdm

Typed Google Nest Smart Device Management REST commands and stateless Pub/Sub event handling for Go, with an optional REST pull adapter. Known traits, command inputs/results and event variants have schema-generated models; incoming unknown fields and names remain available for compatibility.

[![Go](https://img.shields.io/github/go-mod/go-version/portpowered/go-google-nest-sdm)](go.mod)
[![CI](https://github.com/portpowered/go-google-nest-sdm/actions/workflows/ci.yml/badge.svg)](https://github.com/portpowered/go-google-nest-sdm/actions/workflows/ci.yml)
[![Replay coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fportpowered.github.io%2Fgo-google-nest-sdm%2Fcoverage-replay.json)](https://portpowered.github.io/go-google-nest-sdm/coverage-replay.html)
[![Release](https://img.shields.io/github/v/release/portpowered/go-google-nest-sdm)](https://github.com/portpowered/go-google-nest-sdm/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/portpowered/go-google-nest-sdm.svg)](https://pkg.go.dev/github.com/portpowered/go-google-nest-sdm/pkg/sdm)
[![License](https://img.shields.io/github/license/portpowered/go-google-nest-sdm)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://portpowered.github.io/go-google-nest-sdm/)

Report and publication destinations are configured; their activation remains part of release acceptance. Initial provider contracts follow published Google documentation with synthetic offline examples, without live-device qualification.

```sh
go get github.com/portpowered/go-google-nest-sdm
```

Complete [Device Access authorization](https://developers.google.com/nest/device-access/authorize), then supply the account token on each request:

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
    "github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

func main() {
    client, err := httptransport.NewClient()
    if err != nil { log.Fatal(err) }
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    result, err := client.ListDevices(ctx, sdm.ListDevicesRequest{
        Auth: sdm.AuthContext{AccessToken: os.Getenv("NEST_ACCESS_TOKEN")},
        Parent: "enterprises/" + os.Getenv("NEST_ENTERPRISE_ID"),
    })
    if err != nil { log.Fatal(err) }
    for _, device := range result.Devices { log.Print(device.Name) }
}
```

The following fragments use `ctx`, `auth`, `name` (a returned device name), `enterprise`, `structureName`, `roomName` and caller-owned credentials. Handle each returned error before using its result or sending a dependent command.

| Authentication | Inline example |
| --- | --- |
| Exchange authorization code | `client.ExchangeToken(ctx, sdm.ExchangeTokenRequest{ClientId: id, ClientSecret: secret, Code: code, RedirectUri: redirectURI})` |
| Refresh credentials | `client.RefreshToken(ctx, sdm.RefreshTokenRequest{ClientId: id, ClientSecret: secret, RefreshToken: refreshToken})` |

| Discovery | Inline example |
| --- | --- |
| List devices | `client.ListDevices(ctx, sdm.ListDevicesRequest{Auth: auth, Parent: enterprise})` |
| Get device | `client.GetDevice(ctx, sdm.GetDeviceRequest{Auth: auth, Name: name})` |
| List structures | `client.ListStructures(ctx, sdm.ListStructuresRequest{Auth: auth, Parent: enterprise})` |
| Get structure | `client.GetStructure(ctx, sdm.GetStructureRequest{Auth: auth, Name: structureName})` |
| List rooms | `client.ListRooms(ctx, sdm.ListRoomsRequest{Auth: auth, Parent: structureName})` |
| Get room | `client.GetRoom(ctx, sdm.GetRoomRequest{Auth: auth, Name: roomName})` |

| Control | Inline example |
| --- | --- |
| Fan timer | `client.SetFanTimer(ctx, sdm.SetFanTimerRequest{Auth: auth, DeviceName: name, Params: sdm.FanSetTimerParams{TimerMode: "OFF"}})` |
| Thermostat Eco | `client.SetThermostatEcoMode(ctx, sdm.SetThermostatEcoModeRequest{Auth: auth, DeviceName: name, Params: sdm.ThermostatEcoSetModeParams{Mode: "OFF"}})` |
| Thermostat mode | `client.SetThermostatMode(ctx, sdm.SetThermostatModeRequest{Auth: auth, DeviceName: name, Params: sdm.ThermostatModeSetModeParams{Mode: "HEAT"}})` |
| Heat setpoint | `client.SetHeat(ctx, sdm.SetHeatRequest{Auth: auth, DeviceName: name, Params: sdm.ThermostatTemperatureSetpointSetHeatParams{HeatCelsius: 20}})` |
| Cool setpoint | `client.SetCool(ctx, sdm.SetCoolRequest{Auth: auth, DeviceName: name, Params: sdm.ThermostatTemperatureSetpointSetCoolParams{CoolCelsius: 25}})` |
| Heat/cool range | `client.SetRange(ctx, sdm.SetRangeRequest{Auth: auth, DeviceName: name, Params: sdm.ThermostatTemperatureSetpointSetRangeParams{HeatCelsius: 20, CoolCelsius: 25}})` |

| Camera | Inline example |
| --- | --- |
| Event image credentials | `client.GenerateImage(ctx, sdm.GenerateImageRequest{Auth: auth, DeviceName: name, Params: sdm.CameraEventImageGenerateImageParams{EventId: cameraEventID}})` |
| Download generated image | `mediaClient.DownloadImage(ctx, sdm.DownloadImageRequest{Auth: sdm.ImageAuthContext{EventToken: imageToken}, URL: imageURL})` |
| Download event clip preview | `mediaClient.DownloadClipPreview(ctx, sdm.DownloadClipPreviewRequest{Auth: auth, URL: previewURL})` |
| Start RTSP | `client.GenerateRtspStream(ctx, sdm.GenerateRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamGenerateRtspStreamParams{}})` |
| Extend RTSP | `client.ExtendRtspStream(ctx, sdm.ExtendRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamExtendRtspStreamParams{StreamExtensionToken: extensionToken}})` |
| Stop RTSP | `client.StopRtspStream(ctx, sdm.StopRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamStopRtspStreamParams{StreamExtensionToken: extensionToken}})` |
| Start WebRTC | `client.GenerateWebRtcStream(ctx, sdm.GenerateWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamGenerateWebRtcStreamParams{OfferSdp: offerSDP}})` |
| Extend WebRTC | `client.ExtendWebRtcStream(ctx, sdm.ExtendWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamExtendWebRtcStreamParams{MediaSessionId: mediaSessionID}})` |
| Stop WebRTC | `client.StopWebRtcStream(ctx, sdm.StopWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamStopWebRtcStreamParams{MediaSessionId: mediaSessionID}})` |

| Events | Inline example |
| --- | --- |
| Decode push/message data | `client.DecodePushEvent(ctx, sdm.DecodePushEventRequest{Body: pushBody})` |
| Handle Pub/Sub message data | `sdm.HandleEvent(ctx, sdm.HandleEventRequest{Data: message.Data}, myEventHandler)` |
| Reconcile caller-owned state | `sdm.Reconcile(sdm.ReconcileRequest{State: state, Event: event})` |
| Open optional account-bound pull session | `client.OpenEventSession(ctx, sdm.OpenEventSessionRequest{Auth: pubSubAuth, Subscription: subscriptionName})` |
| Receive next delivery | `session.Next(ctx)` |
| Read typed event | `delivery.Event()` |
| Acknowledge processed delivery | `delivery.Acknowledge(ctx)` |
| Extend deadline or release with zero | `delivery.ModifyAckDeadline(ctx, sdm.AckDeadlineRequest{Seconds: 60})` |
| Close session | `session.Close()` |

A shared client holds configuration; callers own credential storage and explicit renewal. Pub/Sub uses a separate Cloud bearer with subscription permissions. Inject `httptransport.WithHTTPClient(myHTTPDoer)` for every HTTP edge; base URL options support offline testing. Set context deadlines and inspect typed `*sdm.Error` failures with `errors.As`.

Discover capabilities through `device.SupportsCommand(command)` and optionally preflight with `sdm.CheckCommand(sdm.CheckCommandRequest{Device: device, Command: command})`; these inspect returned traits and stream protocols without network calls. Apply partial event updates without replacing omitted values. Unknown incoming values remain preserved; invalid known payloads remain errors. Commands acknowledge acceptance and are never implicitly retried. Use `media.New(media.WithHTTPClient(myMediaHTTPDoer))` from `pkg/dependencies/media` for image/clip downloads and close returned bodies. Only supply trusted SDM HTTPS URLs and reject redirects in injected media transports. Callers own RTSP/WebRTC connections, renewal and explicit stop. For push/message integration, your application owns delivery authentication and acknowledgement after successful handling; `DecodePushEvent` is stateless. Optional pull sessions require explicit close and acknowledgements.

Read the [customer guides](https://portpowered.github.io/go-google-nest-sdm/docs/guides), [generated REST reference](https://portpowered.github.io/go-google-nest-sdm/docs) and [event reference](https://portpowered.github.io/go-google-nest-sdm/docs/asyncapi/events/receiveEvents). Install the separate [CLI](https://portpowered.github.io/go-google-nest-sdm/docs/guides/cli) with `go install github.com/portpowered/go-google-nest-sdm/cmd/go-google-nest-sdm@latest` after publication.

Contributor evidence, generation and release acceptance live in [contributor verification](docs/contributing.md), [the checklist](docs/checklist.md) and [independent review](docs/review.md).






