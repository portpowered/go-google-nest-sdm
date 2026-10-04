package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

	"github.com/portpowered/go-google-nest-sdm/internal/protocol"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

// Pull receives one batch without acknowledging its messages or extending deadlines.
// The token is a Cloud Pub/Sub credential; the SDM account token is separate.
func (client *Client) Pull(
	ctx context.Context, token, subscription string, input wire.PullRequest,
) (wire.PullResponse, error) {
	var result wire.PullResponse
	if input.MaxMessages <= 0 {
		return result, fail("Pull", ErrorInvalidRequest, nil)
	}

	path, err := resourcePath(
		subscription,
		protocol.PathPull,
		protocol.CollectionProjects,
		protocol.CollectionSubscriptions,
	)
	if err != nil {
		return result, err
	}

	body, err := json.Marshal(input)
	if err != nil {
		return result, fail("Pull", ErrorInvalidRequest, err)
	}

	err = client.exchange(
		ctx,
		"Pull",
		protocol.MethodPull,
		client.pubSubBaseURL+path,
		token,
		protocol.MIMEApplicationJSON,
		bytes.NewReader(body),
		&result,
	)

	return result, err
}

// Acknowledge explicitly acknowledges processed deliveries.
func (client *Client) Acknowledge(
	ctx context.Context, token, subscription string, input wire.AcknowledgeRequest,
) error {
	if !validAckIDs(input.AckIds) {
		return fail("Acknowledge", ErrorInvalidRequest, nil)
	}

	path, err := resourcePath(
		subscription,
		protocol.PathAcknowledge,
		protocol.CollectionProjects,
		protocol.CollectionSubscriptions,
	)
	if err != nil {
		return err
	}

	body, err := json.Marshal(input)
	if err != nil {
		return fail("Acknowledge", ErrorInvalidRequest, err)
	}

	var result wire.EmptyResponse

	return client.exchange(
		ctx,
		"Acknowledge",
		protocol.MethodAcknowledge,
		client.pubSubBaseURL+path,
		token,
		protocol.MIMEApplicationJSON,
		bytes.NewReader(body),
		&result,
	)
}

// ModifyAckDeadline changes the deadline in seconds; zero releases a message for redelivery.
func (client *Client) ModifyAckDeadline(
	ctx context.Context, token, subscription string, input wire.ModifyAckDeadlineRequest,
) error {
	if !validAckIDs(input.AckIds) || input.AckDeadlineSeconds < 0 || input.AckDeadlineSeconds > maxAckDeadlineSeconds {
		return fail("ModifyAckDeadline", ErrorInvalidRequest, nil)
	}

	path, err := resourcePath(
		subscription,
		protocol.PathModifyAckDeadline,
		protocol.CollectionProjects,
		protocol.CollectionSubscriptions,
	)
	if err != nil {
		return err
	}

	body, err := json.Marshal(input)
	if err != nil {
		return fail("ModifyAckDeadline", ErrorInvalidRequest, err)
	}

	var result wire.EmptyResponse

	return client.exchange(
		ctx,
		"ModifyAckDeadline",
		protocol.MethodModifyAckDeadline,
		client.pubSubBaseURL+path,
		token,
		protocol.MIMEApplicationJSON,
		bytes.NewReader(body),
		&result,
	)
}

const maxAckDeadlineSeconds = 600

func validAckIDs(ids []string) bool {
	if len(ids) == 0 {
		return false
	}

	return !slices.Contains(ids, "")
}
