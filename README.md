# go-google-nest-sdm

Typed Google Nest Smart Device Management REST commands and Pub/Sub event consumption for Go. Known traits, command inputs/results and event variants have schema-generated models; incoming unknown fields and names remain available for compatibility.

[![Go](https://img.shields.io/github/go-mod/go-version/portpowered/go-google-nest-sdm)](go.mod)
[![CI](https://github.com/portpowered/go-google-nest-sdm/actions/workflows/ci.yml/badge.svg)](https://github.com/portpowered/go-google-nest-sdm/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fportpowered.github.io%2Fgo-google-nest-sdm%2Fcoverage.json)](https://portpowered.github.io/go-google-nest-sdm/coverage.html)
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
        Auth: sdm.AuthContext{AccessToken: os.Getenv("SDM_ACCESS_TOKEN")},
        Parent: "enterprises/" + os.Getenv("SDM_PROJECT_ID"),
    })
    if err != nil { log.Fatal(err) }
    for _, device := range result.Devices { log.Print(device.Name) }
}
```

The following fragments use `ctx`, `auth`, `name` (a returned device name), `enterprise`, `structureName`, `roomName` and caller-owned credentials. Handle each returned error before using its result or sending a dependent command.

| Authentication | Inline example |
| --- | --- |
| Exchange authorization code | `client.(sdm.AuthClient).ExchangeToken(ctx, sdm.ExchangeTokenRequest{ClientId: id, ClientSecret: secret, Code: code, RedirectUri: redirectURI})` |
| Refresh credentials | `client.(sdm.AuthClient).RefreshToken(ctx, sdm.RefreshTokenRequest{ClientId: id, ClientSecret: secret, RefreshToken: refreshToken})` |

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
| Start RTSP | `client.GenerateRtspStream(ctx, sdm.GenerateRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamGenerateRtspStreamParams{}})` |
| Extend RTSP | `client.ExtendRtspStream(ctx, sdm.ExtendRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamExtendRtspStreamParams{StreamExtensionToken: extensionToken}})` |
| Stop RTSP | `client.StopRtspStream(ctx, sdm.StopRtspStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamStopRtspStreamParams{StreamExtensionToken: extensionToken}})` |
| Start WebRTC | `client.GenerateWebRtcStream(ctx, sdm.GenerateWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamGenerateWebRtcStreamParams{OfferSdp: offerSDP}})` |
| Extend WebRTC | `client.ExtendWebRtcStream(ctx, sdm.ExtendWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamExtendWebRtcStreamParams{MediaSessionId: mediaSessionID}})` |
| Stop WebRTC | `client.StopWebRtcStream(ctx, sdm.StopWebRtcStreamRequest{Auth: auth, DeviceName: name, Params: sdm.CameraLiveStreamStopWebRtcStreamParams{MediaSessionId: mediaSessionID}})` |

| Events | Inline example |
| --- | --- |
| Open account-bound pull session | `client.(sdm.EventClient).OpenEventSession(ctx, sdm.OpenEventSessionRequest{Auth: pubSubAuth, Subscription: subscriptionName})` |
| Receive next delivery | `session.Next(ctx)` |
| Read typed event | `delivery.Event()` |
| Acknowledge processed delivery | `delivery.Acknowledge(ctx)` |
| Extend deadline or release with zero | `delivery.ModifyAckDeadline(ctx, sdm.AckDeadlineRequest{Seconds: 60})` |
| Close session | `session.Close()` |

A shared client holds configuration; callers own credential storage and explicit renewal. Pub/Sub uses a separate Cloud bearer with subscription permissions. Inject `httptransport.WithHTTPClient(myHTTPDoer)` for every HTTP edge; base URL options support offline testing. Set context deadlines and inspect typed `*sdm.Error` failures with `errors.As`.

Discover capabilities through returned traits and stream protocols. Apply partial event updates without replacing omitted values. Unknown incoming values remain preserved; invalid known payloads remain errors. Commands acknowledge acceptance and are never implicitly retried. Callers own image retrieval, RTSP/WebRTC connections, renewal and explicit stop. Event sessions require explicit close and acknowledgements after successful handling.

Read the [customer guides](https://portpowered.github.io/go-google-nest-sdm/docs/guides), [generated REST reference](https://portpowered.github.io/go-google-nest-sdm/docs/openapi) and [event reference](https://portpowered.github.io/go-google-nest-sdm/docs/asyncapi). Install the separate [CLI](docs/guides/cli.mdx) with `go install github.com/portpowered/go-google-nest-sdm/cmd/go-google-nest-sdm@latest` after publication.

Contributor evidence, generation and release acceptance live in [contributor verification](docs/contributing.md), [the checklist](docs/checklist.md) and [independent review](docs/review.md).
