package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

var (
	imageURL      = regexp.MustCompile(protocol.MediaImageURLPattern)
	clipURL       = regexp.MustCompile(protocol.MediaClipURLPattern)
	errInvalidURL = errors.New("invalid provider-issued media URL")
)

// DownloadImage retrieves a trusted GenerateImage URL with its event token.
// The caller owns the returned body. No request is retried or redirected.
func (client *Client) DownloadImage(
	ctx context.Context, input sdm.DownloadImageRequest,
) (sdm.DownloadImageResult, error) {
	const operation = "DownloadImage"

	endpoint, err := checkedURL(input.URL, imageURL)
	if err != nil || !validToken(input.Auth.EventToken) ||
		input.Width != nil && *input.Width < 1 || input.Height != nil && *input.Height < 1 {
		return sdm.DownloadImageResult{}, failure(operation, sdm.ErrorInvalidRequest, err)
	}

	if input.Width != nil || input.Height != nil {
		query, parseErr := url.ParseQuery(endpoint.RawQuery)
		if parseErr != nil {
			return sdm.DownloadImageResult{}, failure(operation, sdm.ErrorInvalidRequest, parseErr)
		}

		if input.Width != nil {
			query.Set(protocol.QueryWidth, strconv.Itoa(*input.Width))
		}

		if input.Height != nil {
			query.Set(protocol.QueryHeight, strconv.Itoa(*input.Height))
		}

		endpoint.RawQuery = query.Encode()
	}

	//nolint:bodyclose // Successful download transfers body ownership to the caller (GO-09).
	response, err := client.download(
		ctx, operation, protocol.MethodDownloadImage, endpoint, protocol.BasicPrefix+input.Auth.EventToken,
	)
	if err != nil {
		return sdm.DownloadImageResult{}, err
	}

	return sdm.DownloadImageResult{
		Body: response.Body, ContentType: response.Header.Get(protocol.HeaderContentType),
		ContentLength: response.ContentLength,
	}, nil
}

// DownloadClipPreview retrieves a trusted ClipPreview URL with OAuth credentials.
// The caller owns the returned body. No request is retried or redirected.
func (client *Client) DownloadClipPreview(
	ctx context.Context, input sdm.DownloadClipPreviewRequest,
) (sdm.DownloadClipPreviewResult, error) {
	const operation = "DownloadClipPreview"

	endpoint, err := checkedURL(input.URL, clipURL)
	if err != nil || !validToken(input.Auth.AccessToken) {
		return sdm.DownloadClipPreviewResult{}, failure(operation, sdm.ErrorInvalidRequest, err)
	}

	//nolint:bodyclose // Successful download transfers body ownership to the caller (GO-09).
	response, err := client.download(
		ctx, operation, protocol.MethodDownloadClipPreview, endpoint, protocol.BearerPrefix+input.Auth.AccessToken,
	)
	if err != nil {
		return sdm.DownloadClipPreviewResult{}, err
	}

	return sdm.DownloadClipPreviewResult{
		Body: response.Body, ContentType: response.Header.Get(protocol.HeaderContentType),
		ContentLength: response.ContentLength,
	}, nil
}

func checkedURL(value string, pattern *regexp.Regexp) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse media URL: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.Fragment != "" || parsed.Opaque != "" || !pattern.MatchString(value) {
		return nil, errInvalidURL
	}

	return parsed, nil
}

func validToken(token string) bool {
	return token != "" && !strings.ContainsAny(token, " \t\r\n")
}

func (client *Client) download(
	ctx context.Context, operation, method string, endpoint *url.URL, authorization string,
) (*http.Response, error) {
	if ctx == nil {
		return nil, failure(operation, sdm.ErrorInvalidRequest, nil)
	}

	err := ctx.Err()
	if err != nil {
		return nil, failure(operation, causeKind(err), err)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), nil)
	if err != nil {
		return nil, failure(operation, sdm.ErrorInvalidRequest, err)
	}

	request.Header.Set(protocol.HeaderAuthorization, authorization)

	response, err := client.httpClient.Do(request)
	if err != nil {
		if response != nil {
			closeBody(response.Body)
		}

		return nil, failure(operation, causeKind(err), err)
	}

	if response == nil || response.Body == nil {
		return nil, failure(operation, sdm.ErrorInvalidResponse, nil)
	}

	if response.StatusCode != http.StatusOK {
		closeBody(response.Body)

		return nil, &sdm.Error{
			Operation: operation, Kind: statusKind(response.StatusCode), StatusCode: response.StatusCode, Cause: nil,
		}
	}

	return response, nil
}

func closeBody(body io.ReadCloser) {
	if body != nil {
		_ = body.Close()
	}
}

func failure(operation string, kind sdm.ErrorKind, cause error) *sdm.Error {
	return &sdm.Error{Operation: operation, Kind: kind, Cause: cause, StatusCode: 0}
}

func causeKind(err error) sdm.ErrorKind {
	if errors.Is(err, context.Canceled) {
		return sdm.ErrorCanceled
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return sdm.ErrorTimeout
	}

	return sdm.ErrorTransport
}

func statusKind(status int) sdm.ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return sdm.ErrorUnauthorized
	case http.StatusNotFound, http.StatusGone:
		return sdm.ErrorNotFound
	case http.StatusTooManyRequests:
		return sdm.ErrorRateLimited
	default:
		if status >= http.StatusInternalServerError {
			return sdm.ErrorServer
		}

		return sdm.ErrorInvalidResponse
	}
}
