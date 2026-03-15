package github

// LookingFor represents what a user is looking for in the pool.
type LookingFor = string

const (
	LookingForFriendship   LookingFor = "friendship"
	LookingForDating       LookingFor = "dating"
	LookingForRelationship LookingFor = "relationship"
	LookingForNetworking   LookingFor = "networking"
	LookingForOpen         LookingFor = "open"
)

// AllLookingForOptions returns all valid LookingFor values.
func AllLookingForOptions() []LookingFor {
	return []LookingFor{
		LookingForFriendship,
		LookingForDating,
		LookingForRelationship,
		LookingForNetworking,
		LookingForOpen,
	}
}

// DatingProfile is the user's dating profile data.
// Serialized to JSON, encrypted, and stored as the payload inside a .bin file.
// The pubkey is NOT in this struct — it's the first 32 bytes of the .bin.
// Status is NOT here — always "open" on registration, managed by the system.
type DatingProfile struct {
	// From GitHub API
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Location    string `json:"location"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Website     string `json:"website,omitempty"`
	Social      []string `json:"social,omitempty"`

	// From {username}/{username}/README.md
	Showcase string `json:"showcase,omitempty"` // base64-encoded markdown

	// From {username}/dating/README.md
	Interests  []string     `json:"interests,omitempty"`
	LookingFor []LookingFor `json:"looking_for,omitempty"`
	About      string       `json:"about,omitempty"`
}

// ProfileField represents a toggleable field in the profile builder UI.
type ProfileField struct {
	Key     string // json field name
	Label   string // display label
	Value   string // display value (truncated for long content)
	Source  string // "github", "identity", "dating"
	Enabled bool   // whether to include in final profile
}
