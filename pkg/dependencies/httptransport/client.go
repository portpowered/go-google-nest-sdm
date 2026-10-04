// Package httptransport implements stateless, injectable SDM, OAuth and Pub/Sub HTTP exchanges.
package httptransport

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
)

// HTTPDoer is the complete network seam used by every outbound exchange.
// Implementations must support concurrent requests and request context cancellation.
// They must keep account credentials isolated and must not retain or replay cookies
// across calls. Configure cookie-free transports before sharing this client.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// ErrCookieJarUnsupported identifies a stateful HTTP client configuration that
// could carry cookies between account requests on a reusable SDM client.
var ErrCookieJarUnsupported = errors.New("cookie jars are unsupported on a shared SDM client")

// Client holds configuration only. Tokens are supplied separately on every call.
// The caller owns the injected HTTP client and its connections.
type Client struct {
	httpClient                              HTTPDoer
	sdmBaseURL, pubSubBaseURL, oauthBaseURL string
}

// Option configures a client before it is shared.
type Option func(*Client) error

// New validates options and creates a client with official service origins.
// Callers supply contexts with deadlines; no exchange is retried or refreshed implicitly.
func New(options ...Option) (*Client, error) {
	client := &Client{
		httpClient:    defaultHTTPClient(),
		sdmBaseURL:    protocol.SDMBaseURL,
		pubSubBaseURL: protocol.PubSubBaseURL,
		oauthBaseURL:  protocol.OAuthBaseURL,
	}

	for _, option := range options {
		if option == nil {
			return nil, fail("New", ErrorInvalidRequest, nil)
		}

		err := option(client)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Transport:     http.DefaultTransport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Jar:           nil,
		Timeout:       0,
	}
}

// WithHTTPClient supplies an offline or production transport for every endpoint.
// A concrete *http.Client must have no cookie jar and is copied before use so
// later changes to its Jar cannot introduce cross-account state. Custom HTTPDoer
// implementations must honor the same account-isolation contract.
func WithHTTPClient(doer HTTPDoer) Option {
	return func(client *Client) error {
		if doer == nil {
			return fail("New", ErrorInvalidRequest, nil)
		}

		if standard, ok := doer.(*http.Client); ok {
			if standard == nil {
				return fail("New", ErrorInvalidRequest, nil)
			}

			if standard.Jar != nil {
				return fail("New", ErrorInvalidRequest, ErrCookieJarUnsupported)
			}

			copyClient := *standard
			doer = &copyClient
		}

		client.httpClient = doer

		return nil
	}
}

// WithSDMBaseURL replaces the SDM root, including its version prefix.
func WithSDMBaseURL(value string) Option {
	return func(client *Client) error {
		base, err := checkedBaseURL(value)
		if err != nil {
			return err
		}

		client.sdmBaseURL = base

		return nil
	}
}

// WithPubSubBaseURL replaces the Pub/Sub root, including its version prefix.
func WithPubSubBaseURL(value string) Option {
	return func(client *Client) error {
		base, err := checkedBaseURL(value)
		if err != nil {
			return err
		}

		client.pubSubBaseURL = base

		return nil
	}
}

// WithOAuthBaseURL replaces the OAuth root.
func WithOAuthBaseURL(value string) Option {
	return func(client *Client) error {
		base, err := checkedBaseURL(value)
		if err != nil {
			return err
		}

		client.oauthBaseURL = base

		return nil
	}
}

func checkedBaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fail("New", ErrorInvalidRequest, err)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fail("New", ErrorInvalidRequest, nil)
	}

	return strings.TrimRight(value, "/"), nil
}
