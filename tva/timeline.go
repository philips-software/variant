package tva

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"

	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
)

func NewTimeline(thanosID string, selectors []string, config clients.Config) (*Timeline, error) {
	session, err := clients.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("NewTimeline: %w", err)
	}
	timeline := &Timeline{
		Session:   session,
		ThanosID:  thanosID,
		Selectors: selectors,
	}
	timeline.startState = timeline.getCurrentPolicies()
	return timeline, nil
}

type Timeline struct {
	sync.Mutex
	*clients.Session
	ThanosID   string
	Targets    []string
	Selectors  []string
	startState []cfnetv1.Policy
}

func (t *Timeline) Reconcile() error {
	// Retrieve all relevant apps
	apps, _, err := t.V3().GetApplications(ccv3.Query{
		Key:    "label_selector",
		Values: t.Selectors,
	})
	if err != nil {
		return err
	}
	fmt.Printf("found %d matching selectors\n", len(apps))
	// Calculate all required policies
	var generatedPolicies []cfnetv1.Policy
	for _, app := range apps {
		policies, _ := t.GeneratePolicies(app)
		generatedPolicies = append(generatedPolicies, policies...)
	}
	desiredState := append(t.startState, generatedPolicies...)
	currentState := t.getCurrentPolicies()
	fmt.Printf("desired: %d, current: %d\n", len(desiredState), len(currentState))
	// Calculate diff
	var toAdd []cfnetv1.Policy
	for _, p := range desiredState {
		found := false
		for _, q := range currentState {
			if policyEqual(p, q) {
				found = true
			}
		}
		if !found {
			toAdd = append(toAdd, p)
		}
	}
	var toPrune []cfnetv1.Policy
	for _, p := range currentState {
		found := false
		for _, q := range desiredState {
			if policyEqual(p, q) {
				found = true
			}
		}
		if !found {
			toPrune = append(toPrune, p)
		}
	}
	fmt.Printf("adding: %d\n", len(toAdd))
	fmt.Printf("removing: %d\n", len(toPrune))
	t.Networking().RemovePolicies(toPrune)
	t.Networking().CreatePolicies(toAdd)
	return nil
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

func (t *Timeline) GeneratePolicies(app resources.Application) ([]cfnetv1.Policy, error) {
	var policies []cfnetv1.Policy

	metadata, err := t.metadataRetrieve(appMetadata, app.GUID)
	if err != nil {
		return policies, fmt.Errorf("metadataRetrieve: %w", err)
	}
	if port := metadata.Annotations["prometheus.exporter.port"]; port != nil {
		portNumber, err := strconv.Atoi(*port)
		if err != nil {
			return policies, err
		}
		policies = append(policies, t.newPolicy(app.GUID, portNumber))
	}
	if port := metadata.Annotations["prometheus.discovery.port"]; port != nil {
		portNumber, err := strconv.Atoi(*port)
		if err != nil {
			return policies, err
		}
		policies = append(policies, t.newPolicy(app.GUID, portNumber))
	}
	return policies, nil
}

func (t *Timeline) getCurrentPolicies() []cfnetv1.Policy {
	policies, _ := t.Networking().ListPolicies(t.ThanosID)
	return policies
}

func (t *Timeline) newPolicy(destination string, port int) cfnetv1.Policy {
	return cfnetv1.Policy{
		Source: cfnetv1.PolicySource{ID: t.ThanosID},
		Destination: cfnetv1.PolicyDestination{
			ID:       destination,
			Protocol: cfnetv1.PolicyProtocolTCP,
			Ports:    cfnetv1.Ports{Start: port, End: port},
		},
	}
}

const (
	labelsKey      = "labels"
	annotationsKey = "annotations"

	orgMetadata           metadataType = "organizations"
	spaceMetadata         metadataType = "spaces"
	buildpackMetadata     metadataType = "buildpacks"
	appMetadata           metadataType = "apps"
	stackMetadata         metadataType = "stacks"
	segmentMetadata       metadataType = "isolation_segments"
	serviceBrokerMetadata metadataType = "service_brokers"
)

func pathMetadata(m metadataType, guid string) string {
	return fmt.Sprintf("/v3/%s/%s", m, guid)
}

func (t *Timeline) metadataRetrieve(m metadataType, guid string) (Metadata, error) {
	client := t.Session.Raw()
	req, err := client.NewRequest("GET", pathMetadata(m, guid), nil)
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
