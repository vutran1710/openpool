package pooldb

import (
	"path/filepath"
	"testing"
	"time"
)

func testDB(t *testing.T) *PoolDB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTempIndex(t *testing.T, profiles []Profile) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range profiles {
		if err := db.InsertProfile(p); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	return path
}

func sampleProfile(hash string) Profile {
	return Profile{
		MatchHash:   hash,
		Filters:     `{"age":25,"city":"Berlin"}`,
		Vector:      []byte{1, 2, 3, 4},
		DisplayName: "User " + hash,
		About:       "About " + hash,
		Bio:         "Bio " + hash,
		UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- Write tests ---

func TestInsertProfile(t *testing.T) {
	db := testDB(t)
	p := sampleProfile("aaa")

	if err := db.InsertProfile(p); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetProfile("aaa")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected profile, got nil")
	}
	if got.DisplayName != p.DisplayName {
		t.Errorf("display_name = %q, want %q", got.DisplayName, p.DisplayName)
	}
	if got.Filters != p.Filters {
		t.Errorf("filters = %q, want %q", got.Filters, p.Filters)
	}
	if string(got.Vector) != string(p.Vector) {
		t.Errorf("vector mismatch")
	}
}

func TestInsertProfile_Upsert(t *testing.T) {
	db := testDB(t)
	p := sampleProfile("aaa")
	if err := db.InsertProfile(p); err != nil {
		t.Fatal(err)
	}

	p.DisplayName = "Updated"
	p.About = "New about"
	if err := db.InsertProfile(p); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetProfile("aaa")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Updated" {
		t.Errorf("display_name = %q, want %q", got.DisplayName, "Updated")
	}
	if got.About != "New about" {
		t.Errorf("about = %q, want %q", got.About, "New about")
	}

	count, err := db.ProfileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestClearProfiles(t *testing.T) {
	db := testDB(t)
	for _, h := range []string{"aaa", "bbb", "ccc"} {
		if err := db.InsertProfile(sampleProfile(h)); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.ClearProfiles(); err != nil {
		t.Fatal(err)
	}

	count, err := db.ProfileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// --- Read tests ---

func TestListProfiles(t *testing.T) {
	db := testDB(t)
	for _, h := range []string{"aaa", "bbb", "ccc"} {
		if err := db.InsertProfile(sampleProfile(h)); err != nil {
			t.Fatal(err)
		}
	}

	profiles, err := db.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Errorf("len = %d, want 3", len(profiles))
	}
}

func TestGetProfile(t *testing.T) {
	db := testDB(t)
	if err := db.InsertProfile(sampleProfile("aaa")); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetProfile("aaa")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected profile, got nil")
	}
	if got.MatchHash != "aaa" {
		t.Errorf("match_hash = %q, want %q", got.MatchHash, "aaa")
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	db := testDB(t)

	got, err := db.GetProfile("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestProfileCount(t *testing.T) {
	db := testDB(t)

	count, err := db.ProfileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	if err := db.InsertProfile(sampleProfile("aaa")); err != nil {
		t.Fatal(err)
	}

	count, err = db.ProfileCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// --- Sync tests ---

func TestSyncFromIndex(t *testing.T) {
	db := testDB(t)
	indexPath := createTempIndex(t, []Profile{
		sampleProfile("aaa"),
		sampleProfile("bbb"),
		sampleProfile("ccc"),
	})

	count, err := db.SyncFromIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("synced = %d, want 3", count)
	}

	profiles, err := db.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Errorf("len = %d, want 3", len(profiles))
	}
}

func TestSyncFromIndex_RemovesDeleted(t *testing.T) {
	db := testDB(t)

	// Initial sync with 3 profiles
	indexPath1 := createTempIndex(t, []Profile{
		sampleProfile("aaa"),
		sampleProfile("bbb"),
		sampleProfile("ccc"),
	})
	if _, err := db.SyncFromIndex(indexPath1); err != nil {
		t.Fatal(err)
	}

	// Second sync with only 1 profile
	indexPath2 := createTempIndex(t, []Profile{
		sampleProfile("bbb"),
	})
	count, err := db.SyncFromIndex(indexPath2)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("synced = %d, want 1", count)
	}

	// aaa and ccc should be gone
	for _, h := range []string{"aaa", "ccc"} {
		got, err := db.GetProfile(h)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("profile %s should have been removed", h)
		}
	}
}

func TestSyncFromIndex_InvalidatesScores(t *testing.T) {
	db := testDB(t)

	// Initial sync
	p := sampleProfile("aaa")
	indexPath1 := createTempIndex(t, []Profile{p})
	if _, err := db.SyncFromIndex(indexPath1); err != nil {
		t.Fatal(err)
	}

	// Save a score for aaa
	if err := db.SaveScore("aaa", 0.9, `{"age":0.8}`, true); err != nil {
		t.Fatal(err)
	}

	// Change the profile and re-sync
	p.Filters = `{"age":30,"city":"Munich"}`
	p.UpdatedAt = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	indexPath2 := createTempIndex(t, []Profile{p})
	if _, err := db.SyncFromIndex(indexPath2); err != nil {
		t.Fatal(err)
	}

	// Score should be invalidated
	var count int
	err := db.db.QueryRow(`SELECT COUNT(*) FROM scores WHERE match_hash = ?`, "aaa").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("score count = %d, want 0 (should be invalidated)", count)
	}
}

// --- Score tests ---

func TestSaveScore(t *testing.T) {
	db := testDB(t)
	if err := db.InsertProfile(sampleProfile("aaa")); err != nil {
		t.Fatal(err)
	}

	if err := db.SaveScore("aaa", 0.85, `{"age":0.9,"city":0.8}`, true); err != nil {
		t.Fatal(err)
	}

	var score float64
	var passed int
	err := db.db.QueryRow(`SELECT total_score, passed FROM scores WHERE match_hash = ?`, "aaa").Scan(&score, &passed)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.85 {
		t.Errorf("total_score = %f, want 0.85", score)
	}
	if passed != 1 {
		t.Errorf("passed = %d, want 1", passed)
	}
}

func TestMarkSeen(t *testing.T) {
	db := testDB(t)

	if err := db.MarkSeen("aaa", "like"); err != nil {
		t.Fatal(err)
	}

	var action string
	err := db.db.QueryRow(`SELECT action FROM seen WHERE match_hash = ?`, "aaa").Scan(&action)
	if err != nil {
		t.Fatal(err)
	}
	if action != "like" {
		t.Errorf("action = %q, want %q", action, "like")
	}
}

func TestGetSeen(t *testing.T) {
	db := testDB(t)

	if err := db.MarkSeen("aaa", "like"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkSeen("bbb", "skip"); err != nil {
		t.Fatal(err)
	}

	seen, err := db.GetSeen()
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Errorf("len = %d, want 2", len(seen))
	}
	if !seen["aaa"] {
		t.Error("aaa should be seen")
	}
	if !seen["bbb"] {
		t.Error("bbb should be seen")
	}
}

func TestListUnseen(t *testing.T) {
	db := testDB(t)

	for _, h := range []string{"aaa", "bbb", "ccc"} {
		if err := db.InsertProfile(sampleProfile(h)); err != nil {
			t.Fatal(err)
		}
	}

	// Mark aaa as seen
	if err := db.MarkSeen("aaa", "skip"); err != nil {
		t.Fatal(err)
	}

	unseen, err := db.ListUnseen()
	if err != nil {
		t.Fatal(err)
	}
	if len(unseen) != 2 {
		t.Errorf("len = %d, want 2", len(unseen))
	}

	hashes := make(map[string]bool)
	for _, p := range unseen {
		hashes[p.MatchHash] = true
	}
	if hashes["aaa"] {
		t.Error("aaa should not be in unseen")
	}
	if !hashes["bbb"] {
		t.Error("bbb should be in unseen")
	}
	if !hashes["ccc"] {
		t.Error("ccc should be in unseen")
	}
}
