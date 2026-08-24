package auth

const DefaultThemeID = "github"

var validThemes = map[string]struct{}{
	"nord":        {},
	"ayu":         {},
	"tokyo-night": {},
	"catppuccin":  {},
	"dracula":     {},
	"github":      {},
	"monokai":     {},
}

// Legacy theme ids from v1 presets map to the closest new palette.
var themeAliases = map[string]string{
	"warm":     "github",
	"paper":    "github",
	"ocean":    "tokyo-night",
	"midnight": "tokyo-night",
}

// PublicAppearance is the instance palette exposed to clients.
type PublicAppearance struct {
	ThemeID string `json:"theme_id"`
}

// NormalizeThemeID returns a known theme id, applying legacy aliases when needed.
func NormalizeThemeID(id string) string {
	if alias, ok := themeAliases[id]; ok {
		id = alias
	}
	if _, ok := validThemes[id]; ok {
		return id
	}
	return DefaultThemeID
}

// ValidThemeID reports whether id is a known theme slug (after alias resolution).
func ValidThemeID(id string) bool {
	id = NormalizeThemeID(id)
	_, ok := validThemes[id]
	return ok
}
