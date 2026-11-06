// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/webhooks"
)

type webhookService struct {
	conv *conversionService
}

var _ proteusv1connect.WebhookServiceHandler = (*webhookService)(nil)

func (s *webhookService) wh() *webhooks.Service { return s.conv.api.webhooks }

// toConnect adds the webhook errors to the conversion error mapping.
func (s *webhookService) toConnect(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, webhooks.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error())
	case errors.Is(err, webhooks.ErrLimit):
		return connect.NewError(connect.CodeResourceExhausted, err.Error())
	}
	return s.conv.toConnect(ctx, err)
}

func (s *webhookService) CreateWebhookEndpoint(ctx context.Context, req *proteusv1.CreateWebhookEndpointRequest) (*proteusv1.CreateWebhookEndpointResponse, error) {
	e, secret, err := s.wh().Create(ctx, auth.Caller(ctx), req)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.CreateWebhookEndpointResponse{Endpoint: e, Secret: secret}, nil
}

func (s *webhookService) GetWebhookEndpoint(ctx context.Context, req *proteusv1.GetWebhookEndpointRequest) (*proteusv1.GetWebhookEndpointResponse, error) {
	e, err := s.wh().Get(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.GetWebhookEndpointResponse{Endpoint: e}, nil
}

func (s *webhookService) ListWebhookEndpoints(ctx context.Context, _ *proteusv1.ListWebhookEndpointsRequest) (*proteusv1.ListWebhookEndpointsResponse, error) {
	list, err := s.wh().List(ctx, auth.Caller(ctx))
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.ListWebhookEndpointsResponse{Endpoints: list}, nil
}

func (s *webhookService) UpdateWebhookEndpoint(ctx context.Context, req *proteusv1.UpdateWebhookEndpointRequest) (*proteusv1.UpdateWebhookEndpointResponse, error) {
	e, err := s.wh().Update(ctx, auth.Caller(ctx), req)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.UpdateWebhookEndpointResponse{Endpoint: e}, nil
}

func (s *webhookService) DeleteWebhookEndpoint(ctx context.Context, req *proteusv1.DeleteWebhookEndpointRequest) (*proteusv1.DeleteWebhookEndpointResponse, error) {
	if err := s.wh().Delete(ctx, auth.Caller(ctx), req.GetId()); err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.DeleteWebhookEndpointResponse{}, nil
}

func (s *webhookService) EnableWebhookEndpoint(ctx context.Context, req *proteusv1.EnableWebhookEndpointRequest) (*proteusv1.EnableWebhookEndpointResponse, error) {
	e, err := s.wh().SetEnabled(ctx, auth.Caller(ctx), req.GetId(), true)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.EnableWebhookEndpointResponse{Endpoint: e}, nil
}

func (s *webhookService) DisableWebhookEndpoint(ctx context.Context, req *proteusv1.DisableWebhookEndpointRequest) (*proteusv1.DisableWebhookEndpointResponse, error) {
	e, err := s.wh().SetEnabled(ctx, auth.Caller(ctx), req.GetId(), false)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.DisableWebhookEndpointResponse{Endpoint: e}, nil
}

func (s *webhookService) RevealWebhookSecret(ctx context.Context, req *proteusv1.RevealWebhookSecretRequest) (*proteusv1.RevealWebhookSecretResponse, error) {
	secret, prev, err := s.wh().Reveal(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	res := &proteusv1.RevealWebhookSecretResponse{Secret: secret}
	if !prev.IsZero() {
		res.PreviousSecretExpireTime = timestamppb.New(prev)
	}
	return res, nil
}

func (s *webhookService) RotateWebhookSecret(ctx context.Context, req *proteusv1.RotateWebhookSecretRequest) (*proteusv1.RotateWebhookSecretResponse, error) {
	e, secret, err := s.wh().Rotate(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.RotateWebhookSecretResponse{Endpoint: e, Secret: secret}, nil
}

func (s *webhookService) SendTestWebhookEvent(ctx context.Context, req *proteusv1.SendTestWebhookEventRequest) (*proteusv1.SendTestWebhookEventResponse, error) {
	d, err := s.wh().SendTest(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.SendTestWebhookEventResponse{Delivery: d}, nil
}

func (s *webhookService) ListWebhookDeliveries(ctx context.Context, req *proteusv1.ListWebhookDeliveriesRequest) (*proteusv1.ListWebhookDeliveriesResponse, error) {
	list, next, err := s.wh().ListDeliveries(ctx, auth.Caller(ctx), req)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.ListWebhookDeliveriesResponse{Deliveries: list, NextPageToken: next}, nil
}

func (s *webhookService) RedeliverWebhookDelivery(ctx context.Context, req *proteusv1.RedeliverWebhookDeliveryRequest) (*proteusv1.RedeliverWebhookDeliveryResponse, error) {
	d, err := s.wh().Redeliver(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.RedeliverWebhookDeliveryResponse{Delivery: d}, nil
}
