package domain

// OAuth event type constants for EventBus integration.
const (
	OAuthEventLinked   = "auth.oauth.linked"
	OAuthEventUnlinked = "auth.oauth.unlinked"
)

// ValidOAuthEvents is a lookup map for valid OAuth event types.
var ValidOAuthEvents = map[string]bool{
	OAuthEventLinked:   true,
	OAuthEventUnlinked: true,
}