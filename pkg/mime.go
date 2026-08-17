package pkg

import (
	"fmt"
	"mime"
	"slices"
)

func ExtensionMatchesMimeType(extension string, mimeType string) (bool, error) {
	extensions, err := mime.ExtensionsByType(mimeType)
	if err != nil {
		return false, fmt.Errorf("invalid mimeType: %w", err)
	}
	return slices.Contains(extensions, extension), nil
}
