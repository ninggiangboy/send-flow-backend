package shared

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	gotemplate "text/template"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
)

func RenderTemplateSource(subject, sourceHTML, sourceText string, data map[string]any) (*domain.RenderResult, error) {
	var warnings []string

	subjectTmpl, err := gotemplate.New("subject").Option("missingkey=error").Parse(subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject parse: %v", domain.ErrSourceInvalid, err)
	}
	htmlTmpl, err := template.New("html").Option("missingkey=error").Parse(strings.TrimSpace(sourceHTML))
	if err != nil {
		return nil, fmt.Errorf("%w: html parse: %v", domain.ErrSourceInvalid, err)
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		if IsMissingKeyError(err) {
			return nil, fmt.Errorf("%w: subject missing key: %v", domain.ErrRenderPayloadInvalid, err)
		}
		return nil, fmt.Errorf("%w: subject execute: %v", domain.ErrRenderContextInvalid, err)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		if IsMissingKeyError(err) {
			return nil, fmt.Errorf("%w: html missing key: %v", domain.ErrRenderPayloadInvalid, err)
		}
		return nil, fmt.Errorf("%w: html execute: %v", domain.ErrRenderContextInvalid, err)
	}

	var textResult string
	if sourceText != "" {
		textTmpl, err := gotemplate.New("text").Option("missingkey=error").Parse(sourceText)
		if err != nil {
			return nil, fmt.Errorf("%w: text parse: %v", domain.ErrSourceInvalid, err)
		}
		var textBuf bytes.Buffer
		if err := textTmpl.Execute(&textBuf, data); err != nil {
			if IsMissingKeyError(err) {
				return nil, fmt.Errorf("%w: text missing key: %v", domain.ErrRenderPayloadInvalid, err)
			}
			return nil, fmt.Errorf("%w: text execute: %v", domain.ErrRenderContextInvalid, err)
		}
		textResult = textBuf.String()
	}

	return &domain.RenderResult{
		Subject:  subjectBuf.String(),
		HTML:     htmlBuf.String(),
		Text:     textResult,
		Warnings: warnings,
	}, nil
}

func ValidateTemplateSource(subject, sourceHTML, sourceText string) error {
	if _, err := gotemplate.New("subject").Option("missingkey=error").Parse(subject); err != nil {
		return fmt.Errorf("%w: subject: %v", domain.ErrSourceInvalid, err)
	}
	if _, err := template.New("html").Option("missingkey=error").Parse(sourceHTML); err != nil {
		return fmt.Errorf("%w: html: %v", domain.ErrSourceInvalid, err)
	}
	if sourceText != "" {
		if _, err := gotemplate.New("text").Option("missingkey=error").Parse(sourceText); err != nil {
			return fmt.Errorf("%w: text: %v", domain.ErrSourceInvalid, err)
		}
	}
	return nil
}

func IsMissingKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "missing value") || strings.Contains(msg, "<nil>") || strings.Contains(msg, "map has no entry for key")
}
