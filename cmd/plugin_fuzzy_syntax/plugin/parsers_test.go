package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailableLanguages(t *testing.T) {
	for ext := range extensionToLanguage {
		id, ok := extensionToLanguageID[ext]
		assert.True(t, ok)
		assert.NotZero(t, id)
	}
	for ext := range extensionToLanguageID {
		lang, ok := extensionToLanguage[ext]
		assert.True(t, ok)
		assert.NotNil(t, lang)
	}
}
