package i18n

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"

	"ztutor/internal/logutil"
)

//go:embed locales
var localeFS embed.FS

// Available lists the supported UI language codes in cycle order.
var Available = []string{"en", "es", "zh", "ar"}

// rtlLangs is the set of language codes that use a right-to-left script.
// Add new RTL languages here; callers check IsRTL() rather than hard-coding codes.
var rtlLangs = map[string]bool{
	"ar": true,
	"he": true,
	"fa": true,
	"ur": true,
}

// Locale holds the active translation strings with English as fallback.
type Locale struct {
	lang string
	data map[string]string
	en   map[string]string
}

func loadFile(lang string) map[string]string {
	data, err := localeFS.ReadFile("locales/" + lang + ".yaml")
	if err != nil {
		logutil.Warn("i18n: missing locale file for %q: %v", lang, err)
		return nil
	}
	var m map[string]string
	if err := yaml.Unmarshal(data, &m); err != nil {
		logutil.Error("i18n: failed to parse locale %q: %v", lang, err)
		return nil
	}
	return m
}

// New returns a Locale for the given language code, falling back to English
// for any missing keys.
func New(lang string) *Locale {
	en := loadFile("en")
	if en == nil {
		en = map[string]string{}
	}
	l := &Locale{lang: lang, en: en}
	if lang == "en" || lang == "" {
		l.data = en
	} else {
		data := loadFile(lang)
		if data != nil {
			l.data = data
		} else {
			l.data = en
			l.lang = "en"
		}
	}
	return l
}

// T looks up key, substitutes args via fmt.Sprintf when provided, and falls
// back to the English value or the key itself if nothing matches.
func (l *Locale) T(key string, args ...any) string {
	s, ok := l.data[key]
	if !ok {
		s, ok = l.en[key]
	}
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Lang returns the active language code.
func (l *Locale) Lang() string { return l.lang }

// IsRTL reports whether the active locale uses a right-to-left script.
// Use this to flip layout direction; the terminal's bidi algorithm handles
// the actual character rendering.
func (l *Locale) IsRTL() bool { return rtlLangs[l.lang] }

// Next cycles to the next available language.
func (l *Locale) Next() *Locale {
	for i, a := range Available {
		if a == l.lang {
			next := Available[(i+1)%len(Available)]
			return New(next)
		}
	}
	return New("en")
}
