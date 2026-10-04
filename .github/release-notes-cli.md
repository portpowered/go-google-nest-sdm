# Google Nest SDM CLI v0.1.0

The standalone CLI uses SDK v0.1.0 for authentication, discovery, typed device controls, camera media commands and event processing.

Install with `go install github.com/portpowered/go-google-nest-sdm/cmd/go-google-nest-sdm@v0.1.0`. Run `go-google-nest-sdm --help` and follow the [CLI guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/cli).

Provide credentials through documented environment variables, stdin or files. Credential export and device changes require explicit commands; ordinary output excludes tokens. Event processing supports cancellation and acknowledges pull deliveries only after successful output.
