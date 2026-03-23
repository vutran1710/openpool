// E2E: test registration (with schema validation) + interest matching via real GitHub Actions
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/message"
)

var passed, failed int

func check(name string, ok bool, detail string) {
	if ok {
		fmt.Printf("  PASS: %s\n", name)
		passed++
	} else {
		fmt.Printf("  FAIL: %s — %s\n", name, detail)
		failed++
	}
}

func main() {
	salt := os.Getenv("POOL_SALT")
	poolURL := os.Getenv("POOL_URL")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	if salt == "" || poolURL == "" || operatorKeyHex == "" {
		log.Fatal("Set POOL_SALT, POOL_URL, OPERATOR_PRIVATE_KEY")
	}

	operatorKey, _ := hex.DecodeString(operatorKeyHex)
	operatorPub := ed25519.PrivateKey(operatorKey).Public().(ed25519.PublicKey)

	fmt.Println("=== Test 1: Registration with schema validation ===")
	testRegistration(poolURL, operatorPub)

	fmt.Println("\n=== Test 2: Registration with INVALID profile (should be rejected) ===")
	testBadRegistration(poolURL, operatorPub)

	fmt.Println("\n=== Test 3: Interest matching via Action ===")
	testInterestMatching(salt, poolURL, operatorKey, operatorPub)

	fmt.Printf("\n=== RESULTS: %d passed, %d failed ===\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func testRegistration(poolURL string, operatorPub ed25519.PublicKey) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	// Create a VALID profile matching pool.yaml
	profile := map[string]any{
		"age":       25,
		"interests": []string{"hiking", "coding"},
		"about":     "Valid E2E test user",
	}
	profileJSON, _ := json.Marshal(profile)
	bin, _ := crypto.PackUserBin(pub, operatorPub, profileJSON)
	blobHex := hex.EncodeToString(bin)

	content := strings.Join([]string{"test-hash", pubHex, blobHex, "test-sig", "github:e2e-reg"}, "\n")
	body := message.Format("registration-request", content)

	fmt.Println("  Creating valid registration issue...")
	out, err := exec.Command("gh", "issue", "create", "--repo", poolURL,
		"--title", "Registration Request", "--label", "registration",
		"--body", body).CombinedOutput()
	check("create issue", err == nil, string(out))

	if err != nil {
		return
	}
	issueURL := strings.TrimSpace(string(out))
	issueNum := extractNumber(issueURL)
	fmt.Printf("  Issue: %s\n", issueURL)

	fmt.Println("  Waiting 60s for Action...")
	time.Sleep(60 * time.Second)

	// Check Action result
	issueJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s", poolURL, issueNum),
		"--jq", "{state,state_reason,locked}").CombinedOutput()
	fmt.Printf("  Issue state: %s", string(issueJSON))

	check("issue closed", strings.Contains(string(issueJSON), `"closed"`), string(issueJSON))
	check("issue locked", strings.Contains(string(issueJSON), `"locked":true`), string(issueJSON))

	// Check for signed comment
	commentsJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s/comments", poolURL, issueNum),
		"--jq", ".[].body").CombinedOutput()
	check("has openpool:registration comment",
		strings.Contains(string(commentsJSON), "openpool:registration"),
		"no registration comment found")
}

func testBadRegistration(poolURL string, operatorPub ed25519.PublicKey) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	// Create an INVALID profile — age=150 (exceeds max 100 in pool.yaml)
	profile := map[string]any{
		"age":       150,
		"interests": []string{"hiking"},
	}
	profileJSON, _ := json.Marshal(profile)
	bin, _ := crypto.PackUserBin(pub, operatorPub, profileJSON)
	blobHex := hex.EncodeToString(bin)

	content := strings.Join([]string{"test-hash", pubHex, blobHex, "test-sig", "github:e2e-bad"}, "\n")
	body := message.Format("registration-request", content)

	fmt.Println("  Creating INVALID registration issue (age=150)...")
	out, err := exec.Command("gh", "issue", "create", "--repo", poolURL,
		"--title", "Registration Request", "--label", "registration",
		"--body", body).CombinedOutput()
	check("create bad issue", err == nil, string(out))

	if err != nil {
		return
	}
	issueURL := strings.TrimSpace(string(out))
	issueNum := extractNumber(issueURL)
	fmt.Printf("  Issue: %s\n", issueURL)

	fmt.Println("  Waiting 60s for Action...")
	time.Sleep(60 * time.Second)

	// Should be rejected + locked as spam
	issueJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s", poolURL, issueNum),
		"--jq", "{state,state_reason,locked}").CombinedOutput()
	fmt.Printf("  Issue state: %s", string(issueJSON))

	check("bad issue closed", strings.Contains(string(issueJSON), `"closed"`), string(issueJSON))
	check("bad issue locked", strings.Contains(string(issueJSON), `"locked":true`), string(issueJSON))
	check("closed as not_planned", strings.Contains(string(issueJSON), `"not_planned"`), string(issueJSON))
}

func testInterestMatching(salt, poolURL string, operatorKey []byte, operatorPub ed25519.PublicKey) {
	runID := fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	repoDir := "/Users/vutran/Works/terminal-dating/dating-test-pool"

	pubA, _, _ := ed25519.GenerateKey(rand.Reader)
	pubB, _, _ := ed25519.GenerateKey(rand.Reader)

	idA := hash(poolURL + ":github:int-a-" + runID)
	binA := short(salt + ":" + idA)
	matchA := short(salt + ":" + binA)
	idB := hash(poolURL + ":github:int-b-" + runID)
	binB := short(salt + ":" + idB)
	matchB := short(salt + ":" + binB)

	// Write .bin files
	os.WriteFile(repoDir+"/users/"+binA+".bin", append([]byte(pubA), make([]byte, 100)...), 0644)
	os.WriteFile(repoDir+"/users/"+binB+".bin", append([]byte(pubB), make([]byte, 100)...), 0644)
	exec.Command("bash", "-c", fmt.Sprintf("cd %s && git add -A && git commit -m 'e2e interest test' && git pull --rebase && git push", repoDir)).Run()
	fmt.Println("  Pushed .bin files")

	// A likes B
	bodyA := interestBody(binA, matchA, "Hi from A!", operatorPub)
	fmt.Println("  Creating interest A→B...")
	exec.Command("gh", "issue", "create", "--repo", poolURL, "--title", matchB, "--label", "interest", "--body", bodyA).Run()

	fmt.Println("  Waiting 20s...")
	time.Sleep(20 * time.Second)

	// B likes A
	bodyB := interestBody(binB, matchB, "Hi from B!", operatorPub)
	fmt.Println("  Creating interest B→A...")
	exec.Command("gh", "issue", "create", "--repo", poolURL, "--title", matchA, "--label", "interest", "--body", bodyB).Run()

	fmt.Println("  Waiting 45s for match detection...")
	time.Sleep(45 * time.Second)

	// Check both closed + locked
	issuesJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues?state=closed&labels=interest&per_page=5", poolURL),
		"--jq", fmt.Sprintf("[.[] | select(.title == \"%s\" or .title == \"%s\") | {number,title,locked}]", matchA, matchB)).CombinedOutput()
	fmt.Printf("  Closed issues: %s\n", strings.TrimSpace(string(issuesJSON)))

	check("interest issues found", len(string(issuesJSON)) > 5, "no issues")
	check("issues locked", strings.Contains(string(issuesJSON), `"locked":true`), string(issuesJSON))
}

func interestBody(binHash, matchHash, greeting string, opPub ed25519.PublicKey) string {
	payload := fmt.Sprintf(`{"author_bin_hash":"%s","author_match_hash":"%s","greeting":"%s"}`, binHash, matchHash, greeting)
	enc, _ := crypto.Encrypt(opPub, []byte(payload))
	return message.Format("interest", base64.StdEncoding.EncodeToString(enc))
}

func extractNumber(url string) string {
	url = strings.TrimSpace(url)
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}
	return url
}

func hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
func short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}
