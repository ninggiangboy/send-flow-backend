package app

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/render"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/template"
)

// Input type aliases — backward compatible with API-layer callers.
type (
	CreateTemplateInput = template.CreateInput
	UpdateTemplateInput = template.UpdateInput
)

// Result type aliases — backward compatible with API-layer callers.
type (
	TemplateResult     = template.CreateResult
	TemplateListResult = template.ListResult
	VersionResult      = template.PublishResult
	VersionListResult  = template.ListVersionsResult
	RenderResult       = render.Result
)
