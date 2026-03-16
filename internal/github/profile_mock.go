package github

import "encoding/base64"

// MockProfiles returns a set of rich, fully-populated profiles for testing and previewing.
func MockProfiles() []DatingProfile {
	return []DatingProfile{
		{
			DisplayName: "Alice Chen",
			Bio:         "Backend engineer who loves distributed systems and coffee",
			Location:    "Berlin, Germany",
			AvatarURL:   "https://avatars.githubusercontent.com/u/10001",
			Website:     "https://alice.dev",
			Social:      []string{"https://twitter.com/alicedev", "https://linkedin.com/in/alicechen"},
			Showcase:    base64.StdEncoding.EncodeToString([]byte("# Alice\n\n🦀 Rust · Go · Distributed Systems\n\n![stats](https://github-readme-stats.vercel.app/api?username=alice)\n\nBuilding the future of decentralized infrastructure.")),
			Interests:   []string{"rust", "distributed-systems", "hiking", "coffee", "open-source", "mechanical-keyboards"},
			Intent:      []Intent{IntentDating, IntentFriendship},
			GenderTarget: []GenderTarget{GenderDev, GenderMen},
			About:       "Looking for someone who understands why I mass-rewrite in Rust at 2am.\n\nI believe the best dates start with `git clone` and end with a merged PR.\n\nWeekdays: coding. Weekends: hiking in Brandenburg with bad coffee.",
		},
		{
			DisplayName: "Bob Martinez",
			Bio:         "Full-stack dev, weekend climber, terrible at cooking",
			Location:    "Munich, Germany",
			AvatarURL:   "https://avatars.githubusercontent.com/u/10002",
			Website:     "https://bobm.io",
			Social:      []string{"https://twitter.com/bobclimbs"},
			Interests:   []string{"typescript", "climbing", "board-games", "vim", "craft-beer"},
			Intent:      []Intent{IntentDating, IntentYolo},
			GenderTarget: []GenderTarget{GenderWomen, GenderNonBinary},
			About:       "I once debugged a production issue while hanging off a cliff. Not recommended.\n\nLooking for someone who won't judge my Vim config (2000+ lines).",
		},
		{
			DisplayName: "Charlie Wu",
			Bio:         "DevOps by day, DJ by night",
			Location:    "Amsterdam, Netherlands",
			Website:     "https://charlie.dj",
			Social:      []string{"https://soundcloud.com/charliewu", "https://twitter.com/charlie_ops"},
			Interests:   []string{"kubernetes", "vinyl", "electronic-music", "cycling", "photography"},
			Intent:      []Intent{IntentFriendship, IntentNetworking},
			GenderTarget: []GenderTarget{GenderDev},
			About:       "My Kubernetes cluster has better uptime than my love life.\n\nLet's grab a coffee and argue about whether Terraform or Pulumi is better.",
		},
		{
			DisplayName: "Diana Okafor",
			Bio:         "Security researcher. I break things so you don't have to",
			Location:    "Hamburg, Germany",
			AvatarURL:   "https://avatars.githubusercontent.com/u/10004",
			Interests:   []string{"ctf", "reverse-engineering", "cats", "manga", "piano"},
			Intent:      []Intent{IntentDating, IntentFriendship},
			GenderTarget: []GenderTarget{GenderWomen, GenderDev},
			About:       "If you can bypass my auth, you can take me on a date.\n\nSeriously though, I'm looking for someone who appreciates a good CVE writeup over dinner.\n\n🐱 Cat mom to two rescue tabbies.",
		},
		{
			// Minimal profile — only required fields
			DisplayName: "eve",
			Bio:         "...",
			Location:    "Somewhere",
		},
	}
}
