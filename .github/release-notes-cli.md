# Google Nest SDM CLI v0.2.0

The standalone CLI uses SDK v0.2.0 for authentication, discovery, typed device controls, camera media commands and event processing.

`auth login --project DEVICE_ACCESS_PROJECT_ID` opens browser consent and receives a registered localhost callback, then validates state, exchanges the bound code and S256 verifier, and completes the initial device-list call. Configure the OAuth Web application's client ID, secret and exact redirect through the documented credential inputs. Cancellation closes the listener and authorization session.

Install with `go install github.com/portpowered/go-google-nest-sdm/cmd/go-google-nest-sdm@v0.2.0`. Run `go-google-nest-sdm --help` and follow the [CLI guide](https://portpowered.github.io/go-google-nest-sdm/docs/guides/cli).

Provide credentials through documented environment variables, stdin or files. Credential export and device changes require explicit commands; ordinary output excludes tokens. Event processing supports cancellation and acknowledges pull deliveries only after successful output.
