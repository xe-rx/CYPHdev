package verify

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xe-rx/CYPHdev/engine"
)

// buildLog writes a small but representative log: events, a gap block, and a signed
// checkpoint over the final head. Returns the public key an auditor would hold.
func buildLog(t *testing.T, dir string) ed25519.PublicKey {
	t.Helper()
	s, err := engine.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	tree, chain := engine.NewTree(), engine.NewChain()
	pub, priv, _ := ed25519.GenerateKey(nil)
	signer := engine.NewSigner(priv)
	now := time.Unix(0, 0).UTC()

	put := func(key, val string, rev int64) engine.Block {
		e := engine.Event{Kind: engine.Put, Key: key, Value: []byte(val), ModRevision: rev}
		tree.Apply(e)
		b, err := chain.Append(e, tree.Root(), now)
		if err != nil {
			t.Fatal(err)
		}
		s.AppendEvent(e)
		s.AppendBlock(b)
		return b
	}

	put("/agreements/UVA", "alice", 1)
	put("/agreements/UVA", "bob", 2)
	g := engine.Gap{From: 2, To: 9}
	gb, _ := chain.AppendGap(g, tree.Root(), now)
	s.AppendGap(g)
	s.AppendBlock(gb)
	last := put("/agreements/VU", "carol", 10)

	cp := signer.Checkpoint(last.Revision, chain.Head(), now)
	s.AppendCheckpoint(cp)
	s.Close()
	return pub
}

func TestVerifyGoodLog(t *testing.T) {
	dir := t.TempDir()
	pub := buildLog(t, dir)

	res, err := Log(dir, pub)
	if err != nil {
		t.Fatalf("valid log rejected: %v", err)
	}
	if res.Blocks != 4 || res.Events != 3 || res.Gaps != 1 || res.Checkpoints != 1 {
		t.Fatalf("unexpected counts: %+v", res)
	}

	// A historical proof anchors to the verified log.
	p, err := engine.ProveAt(dir, "/agreements/UVA", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckProof(dir, p); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
}

func TestVerifyTamperedValue(t *testing.T) {
	dir := t.TempDir()
	pub := buildLog(t, dir)

	// Tamper an event's value: its hash no longer matches the committed block entry hash.
	tamperFirstLine(t, filepath.Join(dir, "events.ndjson"), func(m map[string]any) {
		m["value"] = []byte("mallory")
	})

	if _, err := Log(dir, pub); err == nil {
		t.Fatal("tampered event value was not detected")
	}
}

func TestVerifyBrokenChain(t *testing.T) {
	dir := t.TempDir()
	pub := buildLog(t, dir)

	// Tamper a committed root: the block re-hashes differently, breaking linkage.
	tamperFirstLine(t, filepath.Join(dir, "blocks.ndjson"), func(m map[string]any) {
		root := m["root"].(string)
		m["root"] = "ff" + root[2:]
	})

	if _, err := Log(dir, pub); err == nil {
		t.Fatal("broken chain linkage was not detected")
	}
}

func TestVerifyForgedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	buildLog(t, dir)

	// An auditor holding a different public key must reject the signatures.
	wrongPub, _, _ := ed25519.GenerateKey(nil)
	if _, err := Log(dir, wrongPub); err == nil {
		t.Fatal("checkpoint signature accepted under the wrong public key")
	}
}

func tamperFirstLine(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(data)
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	b, _ := json.Marshal(m)
	lines[0] = string(b)
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, c := range data {
		if c == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}
