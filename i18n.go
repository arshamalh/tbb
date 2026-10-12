package botbase

import (
	"log/slog"
	"strings"
	"sync"
)

// LangNameKey is the conventional translation key that holds a language's
// own display name (e.g. "English 🇬🇧"). It is used by the built-in language
// picker when no explicit name is passed to Register.
const LangNameKey = "lang"

// Translations maps translation keys to localized templates for one language.
// Templates may contain {placeholders} that are filled by Replace / GetL8d.
type Translations map[string]string

// Language describes a registered language.
type Language struct {
	Code string // e.g. "en", "fa"
	Name string // display name shown in the language picker
}

// I18n is a registry of translations keyed by language code. Lookups fall
// back to the default language when a key is missing in the requested one.
// It is safe for concurrent use.
type I18n struct {
	mu       sync.RWMutex
	fallback string
	order    []string
	langs    map[string]Translations
	names    map[string]string
	logger   *slog.Logger
}

// NewI18n creates an empty registry. fallback is the language used when a
// key or a language is missing ("en" when empty).
func NewI18n(fallback string) *I18n {
	if fallback == "" {
		fallback = "en"
	}
	return &I18n{
		fallback: fallback,
		langs:    make(map[string]Translations),
		names:    make(map[string]string),
		logger:   slog.Default(),
	}
}

// SetLogger sets the logger used to report missing keys.
func (i *I18n) SetLogger(l *slog.Logger) {
	if l != nil {
		i.logger = l
	}
}

// Fallback returns the fallback language code.
func (i *I18n) Fallback() string { return i.fallback }

// Register adds (or replaces) the translations of a language. name is the
// display name used by the language picker; when empty, the LangNameKey entry
// of t is used, then the code itself. Languages keep their registration order.
func (i *I18n) Register(code, name string, t Translations) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.langs[code]; !exists {
		i.order = append(i.order, code)
	}
	if name == "" {
		name = t[LangNameKey]
	}
	if name == "" {
		name = code
	}
	i.langs[code] = t
	i.names[code] = name
}

// Has reports whether a language is registered.
func (i *I18n) Has(code string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.langs[code]
	return ok
}

// Languages returns the registered languages in registration order.
func (i *I18n) Languages() []Language {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]Language, 0, len(i.order))
	for _, code := range i.order {
		out = append(out, Language{Code: code, Name: i.names[code]})
	}
	return out
}

// Get returns the raw template of key in lang. When lang is empty or unknown,
// or the key is missing in it, the fallback language is tried. If the key
// cannot be found at all, "[missing: key]" is returned so the gap is visible
// in the chat instead of producing an empty message.
func (i *I18n) Get(lang, key string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if lang == "" {
		lang = i.fallback
	}
	if t, ok := i.langs[lang]; ok {
		if text, ok := t[key]; ok {
			return text
		}
	}
	if lang != i.fallback {
		if t, ok := i.langs[i.fallback]; ok {
			if text, ok := t[key]; ok {
				i.logger.Debug("i18n: falling back", "lang", lang, "key", key, "fallback", i.fallback)
				return text
			}
		}
	}
	i.logger.Warn("i18n: translation key not found", "lang", lang, "key", key)
	return "[missing: " + key + "]"
}

// Texts returns every registered translation of key, one per language that
// defines it. It is used to match localized reply-keyboard buttons.
func (i *I18n) Texts(key string) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var out []string
	for _, code := range i.order {
		if text, ok := i.langs[code][key]; ok {
			out = append(out, text)
		}
	}
	return out
}

// Replace fills {placeholders} in template using key/value pairs:
//
//	Replace("Hi {name}!", "name", "Arsham") // "Hi Arsham!"
//
// Keys are given without braces. A trailing key without a value is ignored.
func Replace(template string, kv ...string) string {
	if len(kv) < 2 {
		return template
	}
	pairs := make([]string, 0, len(kv))
	for j := 0; j+1 < len(kv); j += 2 {
		pairs = append(pairs, "{"+kv[j]+"}", kv[j+1])
	}
	return strings.NewReplacer(pairs...).Replace(template)
}
