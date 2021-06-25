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

const (
	appMetadata metadataType = "apps"
)

type Timeline struct {
	sync.Mutex
	*clients.Session
	ThanosID   string
	Targets    []string
	Selectors  []string
	startState []cfnetv1.Policy
}

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

// Reconcile manages the network-polices
func (t *Timeline) Reconcile() error {
	t.Lock()
	defer t.Unlock()

	// Retrieve all relevant apps
	apps, _, err := t.V3().GetApplications(ccv3.Query{
		Key:    "label_selector",
		Values: t.Selectors,
	})
	if err != nil {
		return err
	}
	fmt.Printf("found %d matching selectors\n", len(apps))

	// Determine the desired state
	var generatedPolicies []cfnetv1.Policy
	for _, app := range apps {
		// Erase an app from startTime if it show up on the timeline
		t.startState = prunePoliciesByDestination(t.startState, app.GUID)
		policies, _ := t.generatePolicies(app)
		generatedPolicies = append(generatedPolicies, policies...)
	}
	desiredState := uniqPolicies(append(t.startState, generatedPolicies...))
	currentState := t.getCurrentPolicies()
	fmt.Printf("desired: %d, current: %d\n", len(desiredState), len(currentState))

	// Calculate add/prune
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

	// Do it
	fmt.Printf("adding: %d\n", len(toAdd))
	fmt.Printf("removing: %d\n", len(toPrune))
	_ = t.Networking().RemovePolicies(toPrune)
	_ = t.Networking().CreatePolicies(toAdd)
	return nil
}

func (t *Timeline) generatePolicies(app resources.Application) ([]cfnetv1.Policy, error) {
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
