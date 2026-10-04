module example.com/sdm-consumer

go 1.24.0

require github.com/portpowered/go-google-nest-sdm v0.0.0

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/portpowered/go-google-nest-sdm => ../..
