package analyser

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"math/rand"
	"slices"
	"strconv"
)

const (
	maxPublic      = 10
	maxPerPlayer   = 3
	minPublicScore = 15
)

func BuildRecap(insights []Insight, demoID, mapName string, rounds int) Recap {
	recap := Recap{
		DemoID:  demoID,
		MapName: mapName,
		Rounds:  rounds,
	}

	scored := make([]Insight, len(insights))
	copy(scored, insights)
	for i := range scored {
		scored[i].Score = Score(scored[i])
		recap.Trace = append(recap.Trace, DebugEvent{
			Stage:   "scored",
			SteamID: scored[i].SteamID,
			Trigger: scored[i].TriggerType,
			Round:   scored[i].Round,
			Message: "assigned score",
			Fields: map[string]any{
				"score": scored[i].Score,
			},
		})
	}

	tieKeys := samePlayerScoreTieKeys(scored, demoID)
	slices.SortFunc(scored, func(a, b Insight) int {
		return compareInsights(a, b, tieKeys)
	})

	seen := make(map[string]bool)
	var candidates []Insight
	for _, ins := range scored {
		key := ins.SteamID + "|" + ins.TriggerType + "|" + strconv.Itoa(ins.Round)
		if seen[key] {
			recap.Dropped = append(recap.Dropped, DroppedInsight{
				Insight: ins,
				Reason:  "duplicate",
			})
			recap.Trace = append(recap.Trace, DebugEvent{
				Stage:   "deduped",
				SteamID: ins.SteamID,
				Trigger: ins.TriggerType,
				Round:   ins.Round,
				Message: "dropped duplicate",
			})
			continue
		}
		seen[key] = true
		candidates = append(candidates, ins)
	}

	playerCount := make(map[string]int)
	for _, ins := range candidates {
		if ins.Score < minPublicScore {
			recap.Dropped = append(recap.Dropped, DroppedInsight{
				Insight: ins,
				Reason:  "below_min_score",
			})
			recap.Trace = append(recap.Trace, DebugEvent{
				Stage:   "dropped_public",
				SteamID: ins.SteamID,
				Trigger: ins.TriggerType,
				Round:   ins.Round,
				Message: "below minimum score",
				Fields: map[string]any{
					"score":     ins.Score,
					"min_score": minPublicScore,
				},
			})
			continue
		}

		if playerCount[ins.SteamID] >= maxPerPlayer {
			recap.Dropped = append(recap.Dropped, DroppedInsight{
				Insight: ins,
				Reason:  "per_player_cap",
			})
			recap.Trace = append(recap.Trace, DebugEvent{
				Stage:   "dropped_public",
				SteamID: ins.SteamID,
				Trigger: ins.TriggerType,
				Round:   ins.Round,
				Message: "per-player cap reached",
				Fields: map[string]any{
					"max_per_player": maxPerPlayer,
				},
			})
			continue
		}

		if len(recap.Public) >= maxPublic {
			recap.Dropped = append(recap.Dropped, DroppedInsight{
				Insight: ins,
				Reason:  "public_cap",
			})
			recap.Trace = append(recap.Trace, DebugEvent{
				Stage:   "dropped_public",
				SteamID: ins.SteamID,
				Trigger: ins.TriggerType,
				Round:   ins.Round,
				Message: "public cap reached",
				Fields: map[string]any{
					"max_public": maxPublic,
				},
			})
			continue
		}

		recap.Public = append(recap.Public, ins)
		playerCount[ins.SteamID]++
		recap.Trace = append(recap.Trace, DebugEvent{
			Stage:   "selected_public",
			SteamID: ins.SteamID,
			Trigger: ins.TriggerType,
			Round:   ins.Round,
			Message: fmt.Sprintf("selected for public recap (score %d)", ins.Score),
			Fields: map[string]any{
				"score": ins.Score,
			},
		})
	}

	if len(recap.Public) > 0 {
		headline := recap.Public[0]
		recap.Headline = &headline
	}

	return recap
}

// samePlayerScoreTieKeys assigns a pseudo-random order within groups of insights
// that share the same player and score. The RNG is seeded from demoID so the
// same demo always picks the same winner among equals.
func samePlayerScoreTieKeys(insights []Insight, demoID string) map[insightKey]uint64 {
	groups := make(map[string][]insightKey)
	for _, ins := range insights {
		key := ins.SteamID + "|" + strconv.Itoa(ins.Score)
		groups[key] = append(groups[key], insightKey{
			SteamID:     ins.SteamID,
			TriggerType: ins.TriggerType,
			Round:       ins.Round,
		})
	}

	rng := rand.New(rand.NewSource(recapSeed(demoID)))
	tieKeys := make(map[insightKey]uint64)
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		for _, key := range members {
			tieKeys[key] = rng.Uint64()
		}
	}
	return tieKeys
}

type insightKey struct {
	SteamID     string
	TriggerType string
	Round       int
}

func insightIdentity(ins Insight) insightKey {
	return insightKey{
		SteamID:     ins.SteamID,
		TriggerType: ins.TriggerType,
		Round:       ins.Round,
	}
}

func recapSeed(demoID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(demoID))
	return int64(h.Sum64())
}

// compareInsights ranks by score descending. When two insights belong to the
// same player and share a score, tieKeys breaks the tie at random (seeded by
// demo ID). Otherwise ties fall back to round, trigger type, then Steam ID.
func compareInsights(a, b Insight, tieKeys map[insightKey]uint64) int {
	if c := cmp.Compare(b.Score, a.Score); c != 0 {
		return c
	}
	if a.SteamID == b.SteamID {
		if ka, kb := tieKeys[insightIdentity(a)], tieKeys[insightIdentity(b)]; ka != kb {
			return cmp.Compare(ka, kb)
		}
	}
	if c := cmp.Compare(a.Round, b.Round); c != 0 {
		return c
	}
	if c := cmp.Compare(a.TriggerType, b.TriggerType); c != 0 {
		return c
	}
	return cmp.Compare(a.SteamID, b.SteamID)
}
