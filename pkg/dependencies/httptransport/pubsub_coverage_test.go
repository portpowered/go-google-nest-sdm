package httptransport_test

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/portpowered/go-google-nest-sdm/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
)

const (
	coveragePubSubToken  = "placeholder"
	coverageSubscription = "projects/coverage/subscriptions/events"
	coverageAckID        = "coverage-ack"
)

func TestPubSubRejectsInvalidRequestsBeforeSending(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	client, err := httptransport.New(httptransport.WithHTTPClient(eventDoer(func(*http.Request) (*http.Response, error) {
		calls.Add(1)

		return eventResponse(`{}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Pull(t.Context(), coveragePubSubToken, "invalid",
		wire.PullRequest{MaxMessages: 1, ReturnImmediately: nil})
	requirePubSubRequestError(t, err)

	_, err = client.Pull(t.Context(), coveragePubSubToken, coverageSubscription,
		wire.PullRequest{MaxMessages: 0, ReturnImmediately: nil})
	requirePubSubRequestError(t, err)

	for _, ids := range [][]string{nil, {""}, {"valid", ""}} {
		err = client.Acknowledge(t.Context(), coveragePubSubToken, coverageSubscription, wire.AcknowledgeRequest{AckIds: ids})
		requirePubSubRequestError(t, err)
	}

	err = client.Acknowledge(t.Context(), coveragePubSubToken, "invalid",
		wire.AcknowledgeRequest{AckIds: []string{coverageAckID}})
	requirePubSubRequestError(t, err)
	err = client.ModifyAckDeadline(t.Context(), coveragePubSubToken, "invalid", wire.ModifyAckDeadlineRequest{
		AckIds: []string{coverageAckID}, AckDeadlineSeconds: 1,
	})
	requirePubSubRequestError(t, err)

	for _, seconds := range []int{-1, 601} {
		err = client.ModifyAckDeadline(t.Context(), coveragePubSubToken, coverageSubscription, wire.ModifyAckDeadlineRequest{
			AckIds: []string{coverageAckID}, AckDeadlineSeconds: seconds,
		})
		requirePubSubRequestError(t, err)
	}

	if calls.Load() != 0 {
		t.Fatal("invalid request performed network work")
	}
}

func requirePubSubRequestError(t *testing.T, err error) {
	t.Helper()

	var failure *httptransport.Error
	if !errors.As(err, &failure) || failure.Kind != httptransport.ErrorInvalidRequest {
		t.Fatalf("expected invalid Pub/Sub request, got %v", err)
	}
}
