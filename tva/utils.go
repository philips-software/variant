package tva

import (
	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
)

func uniqPolicies(policies []cfnetv1.Policy) []cfnetv1.Policy {
	var result []cfnetv1.Policy
	for _, p := range policies {
		count := 0
		for _, c := range result {
			if policyEqual(p, c) {
				count++
			}
		}
		if count == 0 { // Unique
			result = append(result, p)
		}
	}
	return result
}

func prunePoliciesByDestination(policies []cfnetv1.Policy, destID string) []cfnetv1.Policy {
	var result []cfnetv1.Policy
	for _, p := range policies {
		if p.Destination.ID != destID {
			result = append(result, p)
		}
	}
	return result
}

func policyEqual(a, b cfnetv1.Policy) bool {
	if a.Source.ID != b.Source.ID {
		return false
	}
	if a.Destination.Protocol != b.Destination.Protocol {
		return false
	}
	if a.Destination.ID != b.Destination.ID {
		return false
	}
	if a.Destination.Ports.Start != b.Destination.Ports.Start {
		return false
	}
	if a.Destination.Ports.End != b.Destination.Ports.End {
		return false
	}
	return true
}
