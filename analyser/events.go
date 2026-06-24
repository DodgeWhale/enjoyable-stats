package analyser

type Insight struct {
	SteamID     string
	PlayerName  string
	TriggerType string
	Round       int
	Score       int
	Detail      map[string]any
}

type Summary struct {
	MapName       string
	Rounds        int
	CTScore       int
	TScore        int
	FirstHalfCT   int
	FirstHalfT    int
	SecondHalfCT  int
	SecondHalfT   int
	CTClan        string
	TClan         string
	TrackedSide   string // "CT", "T", or "" when undeterminable or mixed
	TrackedScore  int    // 0 when TrackedSide == ""
	OpponentScore int    // 0 when TrackedSide == ""
	Outcome       string // "won", "lost", "tie", or "" when TrackedSide == ""
}

type Recap struct {
	DemoID   string
	Summary  Summary
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
	Summary   Summary
	StateLog  []StateSnapshot
	NameTrace []DebugEvent
}
