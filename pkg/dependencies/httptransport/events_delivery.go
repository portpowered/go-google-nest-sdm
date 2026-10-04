package httptransport

import (
	"context"

	wire "github.com/portpowered/go-google-nest-sdm/pkg/dependencymodels"
	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type eventDelivery struct {
	session *eventSession
	ackID   string
	event   sdm.EventEnvelope
}

func (delivery *eventDelivery) Event() sdm.EventEnvelope { return delivery.event }

func (delivery *eventDelivery) Acknowledge(ctx context.Context) error {
	if ctx == nil {
		return publicError(fail("Acknowledge", ErrorInvalidRequest, nil))
	}

	merged, cancel := delivery.session.operationContext(ctx)
	defer cancel()

	err := delivery.session.ctx.Err()
	if err != nil {
		return sessionContextError("Acknowledge", err)
	}

	err = merged.Err()
	if err != nil {
		return sessionContextError("Acknowledge", err)
	}

	return publicError(delivery.session.transport.Acknowledge(merged, delivery.session.token,
		delivery.session.subscription, wire.AcknowledgeRequest{AckIds: []string{delivery.ackID}}))
}

func (delivery *eventDelivery) ModifyAckDeadline(ctx context.Context, request sdm.AckDeadlineRequest) error {
	if ctx == nil {
		return publicError(fail("ModifyAckDeadline", ErrorInvalidRequest, nil))
	}

	merged, cancel := delivery.session.operationContext(ctx)
	defer cancel()

	err := delivery.session.ctx.Err()
	if err != nil {
		return sessionContextError("ModifyAckDeadline", err)
	}

	err = merged.Err()
	if err != nil {
		return sessionContextError("ModifyAckDeadline", err)
	}

	return publicError(delivery.session.transport.ModifyAckDeadline(merged, delivery.session.token,
		delivery.session.subscription, wire.ModifyAckDeadlineRequest{
			AckIds: []string{delivery.ackID}, AckDeadlineSeconds: request.Seconds,
		}))
}
