package api

import (
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	campaignapp "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	ingestionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	operationsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
)

func GenerateOpenAPIYAML() ([]byte, error) {
	r := chi.NewRouter()
	humaAPI := humachi.New(r, openAPIConfig())
	registerOpenAPIRoutes(humaAPI, r, documentationRouterDeps())
	return humaAPI.OpenAPI().YAML()
}

func documentationRouterDeps() *RouterDeps {
	return &RouterDeps{
		HealthSvc:       platformhealth.NewService(platformhealth.Options{AppName: "sendflow"}),
		AuthSvc:         &identityapp.Service{},
		SenderSvc:       &senderapp.Service{},
		AudienceSvc:     &audienceapp.Service{},
		ContentSvc:      &contentapp.Service{},
		SuppressionSvc:  &suppressionapp.Service{},
		CampaignSvc:     &campaignapp.Service{},
		DeliverySvc:     &deliveryapp.Service{},
		AccessSvc:       &accessapp.Service{},
		IngestionSvc:    &ingestionapp.Service{},
		TrackingSvc:     &trackingapp.Service{},
		AnalyticsSvc:    &analyticsapp.Service{},
		WebhooksSvc:     &webhooksapp.Service{},
		OperationsSvc:   &operationsapp.Service{},
		NotificationSvc: &notificationapp.Service{},
		SettingsSvc:     &identityapp.Service{},
		AuditSvc:        &auditapp.Service{},
		FrontendBaseURL: "http://localhost:3000",
	}
}
