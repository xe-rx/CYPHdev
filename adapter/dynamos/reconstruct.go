package dynamos

import (
	"strings"
	"time"

	"github.com/xe-rx/CYPHdev/engine"
)

type VerdictKind string

const (
	Compliant    VerdictKind = "compliant"
	NonCompliant VerdictKind = "noncompliant"
	Unknown      VerdictKind = "unknown"
)

type Reason string

const (
	ReasonNone            Reason = ""
	ReasonPermitted       Reason = "archetype permitted by governing agreement"
	ReasonNotPermitted    Reason = "archetype not in users allowed archetypes"
	ReasonNoAgreement     Reason = "no agreement for steward at job revision"
	ReasonAmbiguousWindow Reason = "governing agreement changed within window"
	ReasonGap             Reason = "watch gap overlaps the governing interval"
)

type Finding struct {
	Exchange          Exchange
	AgreementKey      string
	GoverningRevision int64
	Verdict           VerdictKind
	Reason            Reason
	Proof             engine.Proof
	Agreement         Agreement
}

func Reconstruct(dir string, window time.Duration) ([]Finding, error) {
	exchanges, err := Exchanges(dir)
	if err != nil {
		return nil, err
	}
	times, err := blockTimes(dir)
	if err != nil {
		return nil, err
	}
	gaps, err := collectGaps(dir)
	if err != nil {
		return nil, err
	}
	changes, err := agreementChanges(dir)
	if err != nil {
		return nil, err
	}

	findings := make([]Finding, 0, len(exchanges))
	for _, x := range exchanges {
		// (scope) only checks the agreement for this steward
		key := agreementKey(x.Steward)
		rc := lastAtOrBefore(changes[key], x.Revision)

		// (scope)gets called in loop, causing full SMT rebuikld, consider caching rebuilt tree
		proof, perr := engine.ProveAt(dir, key, x.Revision)
		if perr != nil {
			return nil, perr
		}

		f := Finding{
			Exchange:          x,
			AgreementKey:      key,
			GoverningRevision: rc,
			Proof:             proof,
		}

		if overlapsGap(gaps, rc, x.Revision) {
			f.Verdict = Unknown
			f.Reason = ReasonGap
			findings = append(findings, f)
			continue
		}

		if !proof.Present {
			f.Verdict = NonCompliant
			f.Reason = ReasonNoAgreement
			findings = append(findings, f)
			continue
		}

		ag, aerr := ParseAgreement(proof.Value)
		if aerr != nil {
			return nil, aerr
		}
		f.Agreement = ag
		if ag.Permits(x.User, x.Archetype) {
			f.Verdict = Compliant
			f.Reason = ReasonPermitted
		} else {
			f.Verdict = NonCompliant
			f.Reason = ReasonNotPermitted
		}
		// (scope) flags any change in window even if verdict same
		if rc > 0 {
			tj, okj := times[x.Revision]
			tc, okc := times[rc]
			if okj && okc && tj.Sub(tc) <= window {
				f.Verdict = Unknown
				f.Reason = ReasonAmbiguousWindow
			}
		}

		findings = append(findings, f)
	}
	return findings, nil
}

func agreementChanges(dir string) (map[string][]int64, error) {
	out := map[string][]int64{}
	for e, err := range engine.ReadEvents(dir) {
		if err != nil {
			return nil, err
		}
		if e.Kind != engine.Put && e.Kind != engine.Snapshot {
			continue
		}
		if !strings.HasPrefix(e.Key, agreementPrefix) {
			continue
		}
		out[e.Key] = append(out[e.Key], e.ModRevision)
	}
	return out, nil
}

func lastAtOrBefore(revs []int64, rj int64) int64 {
	var best int64
	for _, r := range revs {
		if r <= rj && r > best {
			best = r
		}
	}
	return best
}

func blockTimes(dir string) (map[int64]time.Time, error) {
	out := map[int64]time.Time{}
	for b, err := range engine.ReadBlocks(dir) {
		if err != nil {
			return nil, err
		}
		out[b.Revision] = b.Time
	}
	return out, nil
}

func collectGaps(dir string) ([]engine.Gap, error) {
	var out []engine.Gap
	for g, err := range engine.ReadGaps(dir) {
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func overlapsGap(gaps []engine.Gap, lo, hi int64) bool {
	for _, g := range gaps {
		if g.From <= hi && g.To >= lo {
			return true
		}
	}
	return false
}
