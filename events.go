package gotopo

// EventHandlers receive cache and connection changes. Handlers run outside
// internal locks; they should return promptly.
type EventHandlers struct {
	FeatureAdded   func(Feature)
	FeatureUpdated func(Feature)
	FeatureDeleted func(id, class string)
	Disconnected   func(error)
	Reconnected    func()
	Synced         func()
	MapClosed      func()
}
