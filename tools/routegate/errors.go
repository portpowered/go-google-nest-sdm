package main

import "errors"

var errRouteInvalid = errors.New("route inventory violation")

const (
	nilIdentifier      = "nil"
	imageOperation     = "DownloadImage"
	clipOperation      = "DownloadClipPreview"
	httpImport         = "net/http"
	requestConstructor = "NewRequestWithContext"
	exchangeHelper     = "exchange"
	jsonExchangeHelper = "exchangeJSON"
	downloadHelper     = "download"
	resourceHelper     = "resource"
	queryEncoder       = "Encode"
	exchangeArguments  = 8
	mediaArguments     = 5
	keyValueArguments  = 2
)
