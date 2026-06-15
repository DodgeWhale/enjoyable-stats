package analyser

type Insight struct {
	SteamID     string
	TriggerType string
	Round       int
	Detail      map[string]any
}
