package tva

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/percona/promconfig/rules"
)

func UniqApps(apps []resources.Application) []resources.Application {
	var result []resources.Application
	for _, p := range apps {
		count := 0
		for _, c := range result {
			if c.GUID == p.GUID {
				count++
			}
		}
		if count == 0 { // Unique
			result = append(result, p)
		}
	}
	return result
}

func UniqPolicies(policies []cfnetv1.Policy) []cfnetv1.Policy {
	var result []cfnetv1.Policy
	for _, p := range policies {
		count := 0
		for _, c := range result {
			if PolicyEqual(p, c) {
				count++
			}
		}
		if count == 0 { // Unique
			result = append(result, p)
		}
	}
	return result
}

func PrunePoliciesByDestination(policies []cfnetv1.Policy, destID string) []cfnetv1.Policy {
	var result []cfnetv1.Policy
	for _, p := range policies {
		if p.Destination.ID != destID {
			result = append(result, p)
		}
	}
	return result
}

func PolicyEqual(a, b cfnetv1.Policy) bool {
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

func MetadataRetrieve(client *clients.RawClient, guid string) (Metadata, error) {
	req, err := client.NewRequest("GET", pathMetadata("apps", guid), nil)
	if err != nil {
		return Metadata{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			panic(err)
		}
	}()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Metadata{}, err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return Metadata{}, nil
		}
		return Metadata{}, ccerror.RawHTTPStatusError{
			StatusCode:  resp.StatusCode,
			RawResponse: b,
		}
	}

	var metadataReq MetadataRequest
	err = json.Unmarshal(b, &metadataReq)
	if err != nil {
		return Metadata{}, err
	}
	return metadataReq.Metadata, nil
}

func ParseRules(metadata Metadata) ([]rules.RuleNode, error) {
	var foundRules []rules.RuleNode

	rulesJSON := metadata.Annotations[AnnotationRulesJSON]
	if rulesJSON == nil {
		return foundRules, fmt.Errorf("missing annotation '%s'", AnnotationRulesJSON)
	}
	err := json.NewDecoder(bytes.NewBufferString(*rulesJSON)).Decode(&foundRules)
	if err != nil {
		return foundRules, err
	}
	// Add indexed entries as well
	for k, v := range metadata.Annotations {
		if AnnotationRulesIndexJSONRegex.MatchString(k) {
			var rule rules.RuleNode
			err = json.NewDecoder(bytes.NewBufferString(*v)).Decode(&rule)
			if err != nil {
				continue
			}
			foundRules = append(foundRules, rule)
		}
	}

	return foundRules, nil
}
