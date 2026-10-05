# Google Nest SDM SDK v0.2.0

Typed Go APIs cover SDM devices, structures, rooms, all 13 commands, OAuth credential exchange and refresh, camera media, and Pub/Sub deliveries.

This release adds caller-owned account authorization sessions: open PCM consent, validate the callback state, exchange the bound code and S256 verifier, and make Google's required first device-list call. Reusable clients remain stateless; applications own callback endpoints and credential storage.

The generated reference now shows each command's typed parameters and complete request/result examples, plus resource-update, relation and camera-event payloads. Canonical examples are schema-validated in CI.

Handle events synchronously with `sdm.HandleEvent` or decode HTTP push requests in your own endpoint. Your application owns credentials, event acknowledgement and cached state. An optional REST pull adapter exposes explicit delivery and session lifecycles.

Install with `go get github.com/portpowered/go-google-nest-sdm@v0.2.0`. See the [authentication guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/authentication), [event guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/events) and [API reference](https://portpowered.github.io/go-google-nest-sdm/).

Contracts follow Google's documentation and synthetic offline replay tests; live-device qualification is not included.
