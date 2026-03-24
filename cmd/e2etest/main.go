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

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/message"
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

	fmt.Println("\n=== Test 4: Unmatch via Action ===")
	testUnmatch(poolURL, operatorPub)

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
	repoDir := "/Users/vutran/Works/terminal-dating/openpool-base-pool"

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

	// Compute ephemeral hashes (3-day window, matching pool.yaml interest_expiry)
	expiry := 3 * 24 * time.Hour
	ephB := crypto.EphemeralHash(matchB, expiry)
	ephA := crypto.EphemeralHash(matchA, expiry)

	// A likes B
	bodyA := interestBody(binA, matchA, matchB, "Hi from A!", operatorPub)
	fmt.Println("  Creating interest A→B...")
	exec.Command("gh", "issue", "create", "--repo", poolURL, "--title", ephB, "--label", "interest", "--body", bodyA).Run()

	fmt.Println("  Waiting 20s...")
	time.Sleep(20 * time.Second)

	// B likes A
	bodyB := interestBody(binB, matchB, matchA, "Hi from B!", operatorPub)
	fmt.Println("  Creating interest B→A...")
	exec.Command("gh", "issue", "create", "--repo", poolURL, "--title", ephA, "--label", "interest", "--body", bodyB).Run()

	fmt.Println("  Waiting 45s for match detection...")
	time.Sleep(45 * time.Second)

	// Check both closed + locked (search by ephemeral hashes)
	issuesJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues?state=closed&labels=interest&per_page=5", poolURL),
		"--jq", fmt.Sprintf("[.[] | select(.title == \"%s\" or .title == \"%s\") | {number,title,locked}]", ephA, ephB)).CombinedOutput()
	fmt.Printf("  Closed issues: %s\n", strings.TrimSpace(string(issuesJSON)))

	check("interest issues found", len(string(issuesJSON)) > 5, "no issues")
	check("issues locked", strings.Contains(string(issuesJSON), `"locked":true`), string(issuesJSON))
}

func interestBody(binHash, matchHash, targetMatchHash, greeting string, opPub ed25519.PublicKey) string {
	payload := fmt.Sprintf(`{"author_bin_hash":"%s","author_match_hash":"%s","target_match_hash":"%s","greeting":"%s"}`, binHash, matchHash, targetMatchHash, greeting)
	enc, _ := crypto.Encrypt(opPub, []byte(payload))
	return message.Format("interest", base64.StdEncoding.EncodeToString(enc))
}

func testUnmatch(poolURL string, operatorPub ed25519.PublicKey) {
	// Use the match created by Test 3 — find a closed interest issue to get match_hashes
	issuesJSON, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues?state=closed&labels=interest&per_page=2&sort=created&direction=desc", poolURL),
		"--jq", "[.[] | {number,title}]").CombinedOutput()
	fmt.Printf("  Recent closed interests: %s\n", strings.TrimSpace(string(issuesJSON)))

	// We need two match_hashes to unmatch. Use dummy hashes for a fresh unmatch test.
	matchA := "e2e_unmatch_a_" + fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	matchB := "e2e_unmatch_b_" + fmt.Sprintf("%d", time.Now().UnixNano()%100000)

	// Compute pair_hash and create a match file manually
	a, b := matchA, matchB
	if a > b {
		a, b = b, a
	}
	pairHash := short(a + ":" + b)
	repoDir := "/Users/vutran/Works/terminal-dating/openpool-base-pool"

	os.MkdirAll(repoDir+"/matches", 0755)
	matchData := fmt.Sprintf(`{"match_hash_1":"%s","match_hash_2":"%s"}`, matchA, matchB)
	os.WriteFile(repoDir+"/matches/"+pairHash+".json", []byte(matchData), 0644)
	exec.Command("bash", "-c", fmt.Sprintf("cd %s && git add -A && git commit -m 'e2e: add match for unmatch test' && git pull --rebase && git push", repoDir)).Run()
	fmt.Printf("  Created match file: %s\n", pairHash)

	// Create unmatch issue
	unmatchPayload, _ := json.Marshal(map[string]string{
		"author_match_hash": matchA,
		"target_match_hash": matchB,
	})
	enc, _ := crypto.Encrypt(operatorPub, unmatchPayload)
	body := message.Format("unmatch", base64.StdEncoding.EncodeToString(enc))

	fmt.Println("  Creating unmatch issue...")
	out, err := exec.Command("gh", "issue", "create", "--repo", poolURL,
		"--title", "Unmatch Request", "--label", "unmatch",
		"--body", body).CombinedOutput()
	check("create unmatch issue", err == nil, string(out))

	if err != nil {
		return
	}
	issueURL := strings.TrimSpace(string(out))
	issueNum := extractNumber(issueURL)
	fmt.Printf("  Issue: %s\n", issueURL)

	fmt.Println("  Waiting 60s for Action...")
	time.Sleep(60 * time.Second)

	// Check issue state
	issueState, _ := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s", poolURL, issueNum),
		"--jq", "{state,state_reason,locked}").CombinedOutput()
	fmt.Printf("  Issue state: %s", string(issueState))

	check("unmatch issue closed", strings.Contains(string(issueState), `"closed"`), string(issueState))
	check("unmatch issue locked", strings.Contains(string(issueState), `"locked":true`), string(issueState))

	// Verify match file deleted
	exec.Command("bash", "-c", fmt.Sprintf("cd %s && git pull -q", repoDir)).Run()
	_, statErr := os.Stat(repoDir + "/matches/" + pairHash + ".json")
	check("match file deleted", os.IsNotExist(statErr), fmt.Sprintf("match file still exists: %v", statErr))
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
