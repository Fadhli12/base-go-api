package service

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/example/go-api-base/internal/repository"
)

// TemplateEngine handles email template rendering
type TemplateEngine struct {
	templateRepo repository.EmailTemplateRepository
}

// NewTemplateEngine creates a new TemplateEngine instance
func NewTemplateEngine(templateRepo repository.EmailTemplateRepository) *TemplateEngine {
	return &TemplateEngine{templateRepo: templateRepo}
}

// RenderTemplate renders an email template with provided data
func (e *TemplateEngine) RenderTemplate(ctx context.Context, templateName string, data map[string]any) (htmlContent, textContent string, err error) {
	// Find template by name
	tmpl, err := e.templateRepo.FindByName(ctx, templateName)
	if err != nil {
		return "", "", fmt.Errorf("template not found: %w", err)
	}

	// Check if template is active
	if !tmpl.IsActive {
		return "", "", fmt.Errorf("template %s is not active", templateName)
	}

	// Render HTML content
	htmlContent, err = e.renderString(tmpl.HTMLContent, data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render HTML: %w", err)
	}

	// Render text content (optional)
	if tmpl.TextContent != "" {
		textContent, err = e.renderString(tmpl.TextContent, data)
		if err != nil {
			return "", "", fmt.Errorf("failed to render text: %w", err)
		}
	}

	return htmlContent, textContent, nil
}

// RenderString renders a template string with provided data
func (e *TemplateEngine) renderString(templateStr string, data map[string]any) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ValidateTemplate checks if a template is valid
func (e *TemplateEngine) ValidateTemplate(htmlContent, textContent string) error {
	// Validate HTML template
	if htmlContent != "" {
		_, err := template.New("html").Parse(htmlContent)
		if err != nil {
			return fmt.Errorf("invalid HTML template: %w", err)
		}
	}

	// Validate text template (optional)
	if textContent != "" {
		_, err := template.New("text").Parse(textContent)
		if err != nil {
			return fmt.Errorf("invalid text template: %w", err)
		}
	}

	// At least one content type must be present
	if htmlContent == "" && textContent == "" {
		return fmt.Errorf("template must have either HTML or text content")
	}

	return nil
}