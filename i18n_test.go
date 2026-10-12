package botbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestI18n(t *testing.T) {
	i := NewI18n("en")
	i.Register("en", "", Translations{LangNameKey: "English", "hello": "Hello {name}", "only.en": "EN only"})
	i.Register("fa", "فارسی", Translations{"hello": "سلام {name}"})

	assert.True(t, i.Has("fa"))
	assert.False(t, i.Has("de"))

	assert.Equal(t, "سلام {name}", i.Get("fa", "hello"))
	assert.Equal(t, "Hello {name}", i.Get("", "hello"), "empty lang uses fallback")
	assert.Equal(t, "Hello {name}", i.Get("de", "hello"), "unknown lang uses fallback")
	assert.Equal(t, "EN only", i.Get("fa", "only.en"), "missing key falls back")
	assert.Equal(t, "[missing: nope]", i.Get("fa", "nope"))

	assert.Equal(t, []Language{{"en", "English"}, {"fa", "فارسی"}}, i.Languages())
	assert.Equal(t, []string{"Hello {name}", "سلام {name}"}, i.Texts("hello"))

	// Re-registering keeps the original order.
	i.Register("en", "English 🇬🇧", Translations{"hello": "Hi"})
	assert.Equal(t, "English 🇬🇧", i.Languages()[0].Name)
	assert.Len(t, i.Languages(), 2)
}

func TestReplace(t *testing.T) {
	assert.Equal(t, "Hi Arsham, you have 3", Replace("Hi {name}, you have {n}", "name", "Arsham", "n", "3"))
	assert.Equal(t, "Hi {name}", Replace("Hi {name}"))
	assert.Equal(t, "Hi {name}", Replace("Hi {name}", "name"))
}
