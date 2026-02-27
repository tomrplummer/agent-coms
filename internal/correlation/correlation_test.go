package correlation

import "testing"

func TestGenerateRIDValid(t *testing.T) {
	rid, err := GenerateRID()
	if err != nil {
		t.Fatalf("GenerateRID() error = %v", err)
	}
	if !IsValidRID(rid) {
		t.Fatalf("generated RID %q is invalid", rid)
	}
}

func TestExtractRID(t *testing.T) {
	rid, ok := ExtractRID("Please answer [rid:abc_123] now")
	if !ok {
		t.Fatal("expected RID match")
	}
	if rid != "abc_123" {
		t.Fatalf("ExtractRID() = %q, want %q", rid, "abc_123")
	}
}

func TestTextContainsRIDCaseInsensitive(t *testing.T) {
	if !TextContainsRID("[RID:ABC123] hello", "abc123") {
		t.Fatal("expected case-insensitive RID match")
	}
	if TextContainsRID("[rid:xyz]", "abc123") {
		t.Fatal("did not expect mismatched RID")
	}
}
