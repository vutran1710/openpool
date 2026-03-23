// chatsetup: fix User A's .bin file + create User B for chat testing
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

func main() {
	salt := os.Getenv("POOL_SALT")
	opKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	repoDir := os.Getenv("POOL_REPO_DIR")

	if salt == "" || opKeyHex == "" || repoDir == "" {
		fmt.Println("Set POOL_SALT, OPERATOR_PRIVATE_KEY, POOL_REPO_DIR")
		os.Exit(1)
	}

	opKey, _ := hex.DecodeString(opKeyHex)
	opPub := ed25519.PrivateKey(opKey).Public().(ed25519.PublicKey)
	poolRepo := "vutran1710/dating-test-pool"
	relayURL := "ws://localhost:8082"

	// === User A: fix .bin file ===
	pubA, _, _ := crypto.LoadKeyPair(os.Getenv("HOME") + "/.dating/keys")
	idA := sha256Hex(poolRepo + ":github:27060690")
	binA := sha256Short(salt + ":" + idA)
	matchA := sha256Short(salt + ":" + binA)

	profileA, _ := json.Marshal(map[string]any{
		"about": "Author of this thing", "interests": []string{"coding", "music"}, "age": 30,
	})
	binDataA, _ := crypto.PackUserBin(pubA, opPub, profileA)
	os.MkdirAll(repoDir+"/users", 0755)
	os.WriteFile(repoDir+"/users/"+binA+".bin", binDataA, 0644)
	fmt.Printf("User A: bin=%s match=%s\n", binA, matchA)

	// Update User A config
	cfgA := fmt.Sprintf(`active_pool = 'test-pool'
registries = ['https://github.com/vutran1710/dating-test-registry']
active_registry = 'https://github.com/vutran1710/dating-test-registry'

[user]
id_hash = '%s'
display_name = 'Vu Tran'
username = 'vutran1710'
provider = 'github'
provider_user_id = '27060690'
encrypted_token = ''

[[pools]]
name = 'test-pool'
repo = '%s'
operator_public_key = '%s'
relay_url = '%s'
status = 'active'
bin_hash = '%s'
match_hash = '%s'
`, idA, poolRepo, hex.EncodeToString(opPub), relayURL, binA, matchA)
	os.WriteFile(os.Getenv("HOME")+"/.dating/setting.toml", []byte(cfgA), 0600)

	// === User B: create from scratch ===
	home := "/tmp/dating-user-b"
	os.MkdirAll(home+"/keys", 0700)
	pubB, _, _ := crypto.GenerateKeyPair(home + "/keys")

	idB := sha256Hex(poolRepo + ":managed:user-b-v3")
	binB := sha256Short(salt + ":" + idB)
	matchB := sha256Short(salt + ":" + binB)

	profileB, _ := json.Marshal(map[string]any{
		"about": "Chat test user B", "interests": []string{"hiking", "gaming"}, "age": 25,
	})
	binDataB, _ := crypto.PackUserBin(pubB, opPub, profileB)
	os.WriteFile(repoDir+"/users/"+binB+".bin", binDataB, 0644)
	fmt.Printf("User B: bin=%s match=%s\n", binB, matchB)

	// Create match file
	pairH := pairHash(matchA, matchB)
	os.MkdirAll(repoDir+"/matches", 0755)
	matchData, _ := json.Marshal(map[string]any{"match_hash_1": matchA, "match_hash_2": matchB})
	os.WriteFile(repoDir+"/matches/"+pairH+".json", matchData, 0644)

	// Push
	fmt.Println("Pushing to pool repo...")
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"cd %s && git add -A && git commit -m 'e2e: fix user A bin + add user B + match' && git pull --rebase && git push", repoDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// Write User B config
	cfgB := fmt.Sprintf(`active_pool = 'test-pool'
registries = ['https://github.com/vutran1710/dating-test-registry']
active_registry = 'https://github.com/vutran1710/dating-test-registry'

[user]
id_hash = '%s'
display_name = 'Test User B'
username = 'user-b'
provider = 'managed'
provider_user_id = 'user-b-v3'

[[pools]]
name = 'test-pool'
repo = '%s'
operator_public_key = '%s'
relay_url = '%s'
status = 'active'
bin_hash = '%s'
match_hash = '%s'
`, idB, poolRepo, hex.EncodeToString(opPub), relayURL, binB, matchB)
	os.WriteFile(home+"/setting.toml", []byte(cfgB), 0600)

	fmt.Println("\n=== Ready ===")
	fmt.Printf("User A: match=%s\n", matchA)
	fmt.Printf("User B: match=%s\n", matchB)
	fmt.Println("\nStart relay:")
	fmt.Printf("  POOL_URL=%s POOL_SALT=%s PORT=8082 go run ./cmd/relay/\n\n", poolRepo, salt)
	fmt.Println("Then in two terminals:")
	fmt.Printf("  DATING_HOME=%s bin/dating chat %s\n", os.Getenv("HOME")+"/.dating", matchB)
	fmt.Printf("  DATING_HOME=/tmp/dating-user-b bin/dating chat %s\n", matchA)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
func sha256Short(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:16]
}
func pairHash(a, b string) string {
	combined := a + ":" + b
	if a > b {
		combined = b + ":" + a
	}
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])[:12]
}
