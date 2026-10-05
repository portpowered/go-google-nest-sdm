// Package media retrieves encoded SDM event images and clip previews.
package media

import (
	"errors"
	"net/http"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

// HTTPDoer is the network seam for media retrieval. Implementations must honor
// request cancellation and must not follow redirects carrying credentials.
// Implementations must not retain or replay cookies across account requests.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// ErrCookieJarUnsupported identifies a stateful HTTP client configuration that
// could carry cookies between media requests for separate accounts.
var ErrCookieJarUnsupported = errors.New("cookie jars are unsupported on a shared media client")

// Client holds only an injected transport. Each request supplies credentials.
type Client struct {
	httpClient HTTPDoer
}

var _ sdm.MediaClient = (*Client)(nil)

// Option configures a client before concurrent use.
type Option func(*Client) error

// New creates a client that rejects redirects. Callers own request deadlines.
func New(options ...Option) (*Client, error) {
	client := &Client{httpClient: withoutRedirects(&http.Client{
		Transport: http.DefaultTransport, CheckRedirect: nil, Jar: nil, Timeout: 0,
	})}

	for _, option := range options {
		if option == nil {
			return nil, failure("New", sdm.ErrorInvalidRequest, nil)
		}

		err := option(client)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// WithHTTPClient injects every media exchange. A concrete *http.Client is copied
// with redirects disabled; other implementations must obey HTTPDoer's contract.
// Concrete clients with cookie jars are rejected to preserve account isolation.
// The caller owns the transport and its connection pool.
func WithHTTPClient(doer HTTPDoer) Option {
	return func(client *Client) error {
		if doer == nil {
			return failure("New", sdm.ErrorInvalidRequest, nil)
		}

		if standard, ok := doer.(*http.Client); ok {
			if standard == nil {
				return failure("New", sdm.ErrorInvalidRequest, nil)
			}

			if standard.Jar != nil {
				return failure("New", sdm.ErrorInvalidRequest, ErrCookieJarUnsupported)
			}

			doer = withoutRedirects(standard)
		}

		client.httpClient = doer

		return nil
	}
}

func withoutRedirects(client *http.Client) *http.Client {
	copyClient := *client
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &copyClient
}
