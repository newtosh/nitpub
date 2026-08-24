package sitecontent

// Manifest is the parsed site.toml configuration.
type Manifest struct {
	Home       HomeConfig       `toml:"home" json:"home"`
	Archive    ArchiveConfig    `toml:"archive" json:"archive"`
	Search     SearchConfig     `toml:"search" json:"search"`
	Federation FederationConfig `toml:"federation" json:"federation"`
	Content    ContentConfig    `toml:"content" json:"content"`
	Nav        []NavItem        `toml:"nav" json:"nav"`
	Pages      []PageRef        `toml:"pages" json:"pages"`
	Imports    ImportTypes      `toml:"imports" json:"imports,omitempty"`
	Footer     FooterConfig     `toml:"footer" json:"footer"`
	Branding   BrandingConfig   `toml:"branding" json:"branding"`
}

// ContentConfig controls rendering behavior for markdown content (articles,
// notes, custom pages) — separate from FederationConfig since it's about
// how content displays on this site, not about ActivityPub delivery.
type ContentConfig struct {
	// ExternalLinksNewTab nil means true (external links open in a new
	// tab). Only affects links markdown rendering points off-site —
	// same-site/relative links always stay in the current tab regardless
	// of this setting.
	ExternalLinksNewTab *bool `toml:"external_links_new_tab,omitempty" json:"external_links_new_tab"`
}

// ExternalLinksOpenNewTab reports whether markdown-rendered external links
// should open in a new tab.
func (c ContentConfig) ExternalLinksOpenNewTab() bool {
	if c.ExternalLinksNewTab == nil {
		return true
	}
	return *c.ExternalLinksNewTab
}

// BrandingConfig controls the site's uploaded visual identity: the browser
// tab favicon and a brand icon shown beside the site title in the header.
// Both are optional — empty means fall back to the built-in default.
type BrandingConfig struct {
	FaviconURL string `toml:"favicon_url" json:"favicon_url"`
	LogoURL    string `toml:"logo_url" json:"logo_url"`
	// Tagline is the fediverse profile bio (ActivityPub actor summary) —
	// shown by Mastodon and other clients under the display name.
	Tagline string `toml:"tagline" json:"tagline"`
}

// DefaultTagline is used when an operator hasn't set a custom fediverse bio.
const DefaultTagline = "nitpub personal blog"

// NitpubGithubURL is the canonical project repo — deliberately not
// operator-configurable. The footer link exists to draw awareness back
// to the project from every instance running it, not to advertise
// whatever fork/mirror an operator happens to point it at.
const NitpubGithubURL = "https://github.com/newtosh/nitpub"

// FooterConfig controls the site-wide footer shown at the bottom of every
// page: custom text, and a link back to the nitpub project on GitHub an
// operator may want to hide.
type FooterConfig struct {
	Text string `toml:"text" json:"text"`
	// ShowGithubLink nil means true (shown by default — nitpub.com is
	// meant to double as the project's demo site, and every instance
	// linking back helps the project be discovered).
	ShowGithubLink *bool `toml:"show_github_link,omitempty" json:"show_github_link"`
}

// GithubLinkEnabled reports whether the footer should show the GitHub
// project link.
func (f FooterConfig) GithubLinkEnabled() bool {
	if f.ShowGithubLink == nil {
		return true
	}
	return *f.ShowGithubLink
}

// FederationConfig controls ActivityPub cross-posting defaults.
type FederationConfig struct {
	// CrossPostDefault nil means true (share new posts to the fediverse).
	CrossPostDefault *bool `toml:"cross_post_default,omitempty" json:"cross_post_default"`
	// ShowAvatarsDefault nil means true (show reply authors' avatars).
	ShowAvatarsDefault *bool `toml:"show_avatars_default,omitempty" json:"show_avatars_default"`
	// ModerationEnabled nil means true (replies are gated by the pending
	// queue). false auto-approves every incoming reply on arrival, skipping
	// the queue entirely — the blocked-actor list still applies, since
	// blocking is a separate, explicit admin decision from the queue.
	ModerationEnabled *bool `toml:"moderation_enabled,omitempty" json:"moderation_enabled"`
	// RepliesCollapsedDefault nil means true (the replies section starts
	// collapsed on a post page, behind a "Replies (N)" toggle, so a long
	// thread doesn't push the article below the fold).
	RepliesCollapsedDefault *bool `toml:"replies_collapsed_default,omitempty" json:"replies_collapsed_default"`
}

// Enabled reports whether new posts should federate by default.
func (f FederationConfig) Enabled() bool {
	if f.CrossPostDefault == nil {
		return true
	}
	return *f.CrossPostDefault
}

// ModerationOn reports whether incoming replies are gated by the pending
// queue. When false, replies auto-approve on arrival (blocked actors are
// still rejected — see ModerationEnabled).
func (f FederationConfig) ModerationOn() bool {
	if f.ModerationEnabled == nil {
		return true
	}
	return *f.ModerationEnabled
}

// AvatarsEnabled reports whether reply authors' avatars should be shown.
func (f FederationConfig) AvatarsEnabled() bool {
	if f.ShowAvatarsDefault == nil {
		return true
	}
	return *f.ShowAvatarsDefault
}

// RepliesCollapsedByDefault reports whether a post's replies section starts
// collapsed.
func (f FederationConfig) RepliesCollapsedByDefault() bool {
	if f.RepliesCollapsedDefault == nil {
		return true
	}
	return *f.RepliesCollapsedDefault
}

type HomeConfig struct {
	RecentCount int `toml:"recent_count" json:"recent_count"`
}

type ArchiveConfig struct {
	Mode     string `toml:"mode" json:"mode"` // pagination | infinite
	PageSize int    `toml:"page_size" json:"page_size"`
}

type SearchConfig struct {
	Enabled bool `toml:"enabled" json:"enabled"`
}

type NavItem struct {
	Label string `toml:"label" json:"label"`
	Path  string `toml:"path" json:"path"`
	Icon  string `toml:"icon" json:"icon"`
}

type PageRef struct {
	Path string `toml:"path" json:"path"`
	Type string `toml:"type" json:"type"` // markdown | links
	File string `toml:"file" json:"file"`
}

type ImportTypes struct {
	Types map[string]ImportType `toml:"types"`
}

type ImportType struct {
	Extension string `toml:"extension"`
	Handler   string `toml:"handler"`
}

// Page is a resolved custom page ready to serve.
type Page struct {
	Path  string
	Type  string
	Title string
	Body  string      // markdown pages
	Links []LinkEntry // link collection pages
	// File is the backing manifest file (e.g. "pages/contact.md") — set so
	// an authenticated admin viewing the page can deep-link straight into
	// the Site > Page files editor for it. Never exposed to an
	// unauthenticated request (see ServeSitePage).
	File string
}

type LinkEntry struct {
	Title       string `toml:"title" json:"title"`
	URL         string `toml:"url" json:"url"`
	Description string `toml:"description" json:"description"`
	Icon        string `toml:"icon" json:"icon"`
}

type linksFile struct {
	Title string      `toml:"title"`
	Links []LinkEntry `toml:"links"`
}

// Defaults returns manifest defaults when site.toml is missing.
func Defaults() Manifest {
	crossPost := true
	showAvatars := true
	moderationEnabled := true
	repliesCollapsed := true
	showGithubLink := true
	externalLinksNewTab := true
	return Manifest{
		Home: HomeConfig{RecentCount: 0},
		Archive: ArchiveConfig{
			Mode:     "pagination",
			PageSize: 20,
		},
		Search: SearchConfig{Enabled: true},
		Federation: FederationConfig{
			CrossPostDefault:        &crossPost,
			ShowAvatarsDefault:      &showAvatars,
			ModerationEnabled:       &moderationEnabled,
			RepliesCollapsedDefault: &repliesCollapsed,
		},
		Content: ContentConfig{
			ExternalLinksNewTab: &externalLinksNewTab,
		},
		Footer: FooterConfig{
			Text:           "Self-hosted notes & articles",
			ShowGithubLink: &showGithubLink,
		},
	}
}
