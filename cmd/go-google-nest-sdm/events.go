package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/portpowered/go-google-nest-sdm/pkg/sdm"
)

type streamedOutput struct{}

func (app application) events(
	ctx context.Context, operation string, options invocation, account credentials,
) (any, error) {
	if operation == "decode" {
		data, readErr := readParams[json.RawMessage](options.paramsPath, app.in)
		if readErr != nil {
			return nil, readErr
		}

		return sdkResult(sdm.DecodeEvent(data))
	}

	if operation != "pull" {
		return nil, errEventOperation
	}

	return streamedOutput{}, app.pullEvents(ctx, options, account)
}

func (app application) pullEvents(ctx context.Context, options invocation, account credentials) (err error) {
	client, ok := app.client.(sdm.EventClient)
	if !ok {
		return errEventsUnsupported
	}

	if account.AccessToken == "" || options.resource == "" {
		return errEventCredentials
	}

	maxMessages := 1

	session, err := client.OpenEventSession(ctx, sdm.OpenEventSessionRequest{
		Auth:         sdm.AuthContext{AccessToken: account.AccessToken},
		Subscription: options.resource, MaxMessages: &maxMessages,
	})
	if err != nil {
		return wrapError(err)
	}

	defer func() {
		closeErr := session.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close event session: %w", closeErr))
		}
	}()

	for range options.count {
		delivery, nextErr := session.Next(ctx)
		if nextErr != nil {
			return wrapError(nextErr)
		}

		writeErr := json.NewEncoder(app.out).Encode(delivery.Event())
		if writeErr != nil {
			return wrapError(writeErr)
		}

		ackErr := delivery.Acknowledge(ctx)
		if ackErr != nil {
			return wrapError(ackErr)
		}
	}

	return nil
}
