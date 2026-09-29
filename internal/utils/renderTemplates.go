package utils

import (
	"bytes"
	"fmt"
	"html/template"
)

func RenderTemplate(
	templatePath string,
	data any,
) (string, error) {

	tmpl, err := template.ParseFiles(templatePath)

	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf(
			"execute email template: %w",
			err,
		)
	}

	return buf.String(), nil
}
