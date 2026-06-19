package analyser

type Insight struct {
	SteamID     string
	PlayerName  string
	TriggerType string
	Round       int
	Score       int
	Detail      map[string]any
}

type Recap struct {
	DemoID   string
	MapName  string
	Rounds   int
	Headline *Insight
	Public   []Insight
	Dropped  []DroppedInsight
	Trace    []DebugEvent
}

type DroppedInsight struct {
	Insight Insight
	Reason  string
}

type DebugEvent struct {
	Stage   string
	SteamID string
	Trigger string
	Round   int
	Message string
	Fields  map[string]any
}

type Result struct {
	Insights  []Insight
	MapName   string
	Rounds    int
	StateLog  []StateSnapshot
	NameTrace []DebugEvent
}
