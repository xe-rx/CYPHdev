package dynamos

import "encoding/json"

const agreementPrefix = "/policyEnforcer/agreements/"

type Relation struct {
	ID                      string   `json:"ID"`
	RequestTypes            []string `json:"requestTypes"`
	DataSets                []string `json:"dataSets"`
	AllowedArchetypes       []string `json:"allowedArchetypes"`
	AllowedComputeProviders []string `json:"allowedComputeProviders"`
}

type Agreement struct {
	Name             string              `json:"name"`
	Relations        map[string]Relation `json:"relations"`
	ComputeProviders []string            `json:"computeProviders"`
	Archetypes       []string            `json:"archetypes"`
}

func agreementKey(steward string) string {
	return agreementPrefix + steward
}

func ParseAgreement(value []byte) (Agreement, error) {
	var a Agreement
	err := json.Unmarshal(value, &a)
	return a, err
}

func (a Agreement) Permits(user, archetype string) bool {
	rel, ok := a.Relations[user]
	if !ok {
		return false
	}
	supported := false
	for _, s := range a.Archetypes {
		if s == archetype {
			supported = true
			break
		}
	}
	if !supported {
		return false
	}
	for _, allowed := range rel.AllowedArchetypes {
		if allowed == archetype {
			return true
		}
	}
	return false
}
