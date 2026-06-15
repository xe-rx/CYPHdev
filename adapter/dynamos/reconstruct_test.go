package dynamos

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/xe-rx/CYPHdev/engine"
)

type rec struct {
	ev engine.Event
	t  time.Time
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func writeLog(t *testing.T, dir string, priv ed25519.PrivateKey, recs []rec) {
	t.Helper()
	tree := engine.NewTree()
	chain := engine.NewChain()
	store, err := engine.OpenStore(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	signer := engine.NewSigner(priv)
	for _, r := range recs {
		if err := tree.Apply(r.ev); err != nil {
			t.Fatalf("apply: %v", err)
		}
		blk, err := chain.Append(r.ev, tree.Root(), r.t)
		if err != nil {
			t.Fatalf("append block: %v", err)
		}
		if err := store.AppendEvent(r.ev); err != nil {
			t.Fatalf("append event: %v", err)
		}
		if err := store.AppendBlock(blk); err != nil {
			t.Fatalf("append block log: %v", err)
		}
	}
	cp := signer.Checkpoint(chain.LastRev(), chain.Head(), recs[len(recs)-1].t)
	if err := store.AppendCheckpoint(cp); err != nil {
		t.Fatalf("append checkpoint: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
}

func scenario(t *testing.T) []rec {
	base := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	agr1 := mustJSON(t, Agreement{Name: "UVA", Relations: map[string]Relation{
		"alice": {AllowedArchetypes: []string{"computeToData"}},
	}})
	agr2 := mustJSON(t, Agreement{Name: "UVA", Relations: map[string]Relation{
		"alice": {AllowedArchetypes: []string{"computeToData"}},
		"carol": {AllowedArchetypes: []string{"dataThroughTtp"}},
	}})
	jobA := mustJSON(t, CompositionRequest{ArchetypeID: "computeToData", User: User{UserName: "alice"}, JobName: "job-a"})
	jobB := mustJSON(t, CompositionRequest{ArchetypeID: "computeToData", User: User{UserName: "bob"}, JobName: "job-b"})
	jobC := mustJSON(t, CompositionRequest{ArchetypeID: "computeToData", User: User{UserName: "alice"}, JobName: "job-c"})

	return []rec{
		{engine.Event{Kind: engine.Put, Key: "/policyEnforcer/agreements/UVA", Value: agr1, ModRevision: 1}, base},
		{engine.Event{Kind: engine.Put, Key: "/agents/jobs/UVA/alice/job-a", Value: jobA, ModRevision: 2}, base.Add(time.Hour)},
		{engine.Event{Kind: engine.Put, Key: "/agents/jobs/UVA/bob/job-b", Value: jobB, ModRevision: 3}, base.Add(2 * time.Hour)},
		{engine.Event{Kind: engine.Put, Key: "/policyEnforcer/agreements/UVA", Value: agr2, ModRevision: 4}, base.Add(3 * time.Hour)},
		{engine.Event{Kind: engine.Put, Key: "/agents/jobs/UVA/alice/job-c", Value: jobC, ModRevision: 5}, base.Add(3*time.Hour + time.Second)},
		{engine.Event{Kind: engine.Put, Key: "/agents/jobs/UVA/queueInfo/foo", Value: []byte(`"local-x"`), ModRevision: 6}, base.Add(4 * time.Hour)},
	}
}

func TestReconstruct(t *testing.T) {
	dir := t.TempDir()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	writeLog(t, dir, priv, scenario(t))

	findings, err := Reconstruct(dir, 5*time.Second)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 exchanges (queueInfo excluded), got %d", len(findings))
	}

	byJob := map[string]Finding{}
	for _, f := range findings {
		byJob[f.Exchange.Request.JobName] = f
	}

	if got := byJob["job-a"].Verdict; got != Compliant {
		t.Errorf("job-a: want compliant, got %s (%s)", got, byJob["job-a"].Reason)
	}
	if got := byJob["job-b"].Verdict; got != NonCompliant {
		t.Errorf("job-b: want non-compliant, got %s (%s)", got, byJob["job-b"].Reason)
	}
	if got := byJob["job-c"].Verdict; got != Unknown {
		t.Errorf("job-c: want unknown, got %s (%s)", got, byJob["job-c"].Reason)
	}
	if r := byJob["job-c"].Reason; r != ReasonAmbiguousWindow {
		t.Errorf("job-c: want ambiguous-window reason, got %q", r)
	}
}

func TestCertificateVerify(t *testing.T) {
	dir := t.TempDir()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	writeLog(t, dir, priv, scenario(t))

	findings, err := Reconstruct(dir, 5*time.Second)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}

	for _, f := range findings {
		c := Certify(f)
		res, err := VerifyCertificate(dir, pub, c)
		if err != nil {
			t.Fatalf("verify %s: %v", f.Exchange.Request.JobName, err)
		}
		if !res.Valid() {
			t.Errorf("%s: certificate should be valid, got %+v", f.Exchange.Request.JobName, res)
		}
	}
}

func TestCertificateTamperDetected(t *testing.T) {
	dir := t.TempDir()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	writeLog(t, dir, priv, scenario(t))

	findings, err := Reconstruct(dir, 5*time.Second)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}

	var c Certificate
	for _, f := range findings {
		if f.Exchange.Request.JobName == "job-a" {
			c = Certify(f)
		}
	}
	c.Proof.Value = []byte(`{"name":"UVA","relations":{"alice":{"allowedArchetypes":["forged"]}}}`)

	res, err := VerifyCertificate(dir, pub, c)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.RootRecomputed {
		t.Error("tampered value should not recompute to the committed root")
	}
	if res.Valid() {
		t.Error("tampered certificate must not verify")
	}
}
