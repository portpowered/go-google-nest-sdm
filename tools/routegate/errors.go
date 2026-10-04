package main

import "errors"

var errRouteInvalid = errors.New("route inventory violation")

const (
	httpImport         = "net/http"
	requestConstructor = "NewRequestWithContext"
	exchangeHelper     = "exchange"
	downloadHelper     = "download"
	resourceHelper     = "resource"
	queryEncoder       = "Encode"
	exchangeArguments  = 8
	mediaArguments     = 5
	keyValueArguments  = 2
)
