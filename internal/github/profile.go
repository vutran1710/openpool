package github

// Intent represents what a user is looking for.
type Intent = string

const (
	IntentDating      Intent = "dating"
	IntentFriendship  Intent = "friendship"
	IntentYolo        Intent = "yolo"
	IntentNetworking  Intent = "networking"
)

// AllIntentOptions returns all valid Intent values.
func AllIntentOptions() []Intent {
	return []Intent{IntentDating, IntentFriendship, IntentYolo, IntentNetworking}
}

// GenderTarget represents who a user is interested in.
type GenderTarget = string

const (
	GenderMen       GenderTarget = "men"
	GenderWomen     GenderTarget = "women"
	GenderNonBinary GenderTarget = "non-binary"
	GenderDev       GenderTarget = "dev"
)

// AllGenderTargetOptions returns all valid GenderTarget values.
func AllGenderTargetOptions() []GenderTarget {
	return []GenderTarget{GenderMen, GenderWomen, GenderNonBinary, GenderDev}
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

	// From {username}/dating/{name}.md
	Interests    []string       `json:"interests,omitempty"`
	Intent       []Intent       `json:"intent,omitempty"`
	GenderTarget []GenderTarget `json:"gender_target,omitempty"`
	About        string         `json:"about,omitempty"`
}

// ProfileField represents a toggleable field in the profile builder UI.
type ProfileField struct {
	Key     string // json field name
	Label   string // display label
	Value   string // display value (truncated for long content)
	Source  string // "github", "identity", "dating"
	Enabled bool   // whether to include in final profile
}
