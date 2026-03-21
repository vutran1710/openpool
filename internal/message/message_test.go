package message

import "testing"

func TestFormat_Roundtrip(t *testing.T) {
	msg := Format("registration", "some-content-here")
	blockType, content, err := Parse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if blockType != "registration" {
		t.Fatalf("blockType = %q, want registration", blockType)
	}
	if content != "some-content-here" {
		t.Fatalf("content = %q", content)
	}
}

func TestFormat_MultilineContent(t *testing.T) {
	msg := Format("registration-request", "pubkey123\nblobhex456")
	blockType, content, err := Parse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if blockType != "registration-request" {
		t.Fatalf("blockType = %q", blockType)
	}
	if content != "pubkey123\nblobhex456" {
		t.Fatalf("content = %q", content)
	}
}

func TestParse_InvalidFormat(t *testing.T) {
	_, _, err := Parse("just some random text")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestParse_NoFencedBlock(t *testing.T) {
	_, _, err := Parse("<!-- openpool:registration -->")
	if err == nil {
		t.Fatal("expected error for missing fenced block")
	}
}

func TestParse_EmptyContent(t *testing.T) {
	msg := Format("error", "")
	blockType, content, err := Parse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if blockType != "error" {
		t.Fatalf("blockType = %q", blockType)
	}
	if content != "" {
		t.Fatalf("content should be empty, got %q", content)
	}
}

func TestFormat_OutputShape(t *testing.T) {
	msg := Format("match", "blob.sig")
	expected := "<!-- openpool:match -->\n```\nblob.sig\n```"
	if msg != expected {
		t.Fatalf("got:\n%s\nwant:\n%s", msg, expected)
	}
}

func TestParse_WithSurroundingText(t *testing.T) {
	raw := "some noise\n<!-- openpool:registration -->\n```\ncontent\n```\nmore noise"
	blockType, content, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if blockType != "registration" || content != "content" {
		t.Fatalf("got type=%q content=%q", blockType, content)
	}
}
