module example.com/sdm-consumer

go 1.24.0

require github.com/portpowered/go-google-nest-sdm v0.0.0

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/oapi-codegen/runtime v1.7.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.32.0 // indirect
)

replace github.com/portpowered/go-google-nest-sdm => ../..
