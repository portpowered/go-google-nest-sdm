# Google Nest SDM SDK v0.1.0

Typed Go APIs cover SDM devices, structures, rooms, all 13 commands, OAuth credential exchange and refresh, camera media, and Pub/Sub deliveries.

Handle events synchronously with `sdm.HandleEvent` or decode HTTP push requests in your own endpoint. Your application owns credentials, event acknowledgement and cached state. An optional REST pull adapter exposes explicit delivery and session lifecycles.

Install with `go get github.com/portpowered/go-google-nest-sdm@v0.1.0`. See the [authentication guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/authentication), [event guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/events) and [API reference](https://portpowered.github.io/go-google-nest-sdm/).

Contracts follow Google's documentation and synthetic offline replay tests; live-device qualification is not included.
