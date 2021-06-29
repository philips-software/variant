package tva

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

const (
	appMetadata metadataType = "apps"
)

type Config struct {
	clients.Config
	PrometheusConfig string
	InternalDomainID string
	ThanosID         string
	ThanosURL        string
}

type Timeline struct {
	sync.Mutex
	*clients.Session
	targets     []promconfig.ScrapeConfig
	Selectors   []string
	startState  []cfnetv1.Policy
	startConfig *promconfig.Config
	config      Config
}

type ScrapeEndpoint struct {
	ID   string `json:"id"`
	Port int    `json:"port"`
	Host string `json:"host"`
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
}

func NewTimeline(config Config, selectors []string) (*Timeline, error) {
	session, err := clients.NewSession(config.Config)
	if err != nil {
		return nil, fmt.Errorf("NewTimeline: %w", err)
	}
	timeline := &Timeline{
		Session:   session,
		Selectors: selectors,
		config:    config,
	}
	data, err := ioutil.ReadFile(config.PrometheusConfig)
	if err != nil {
		return nil, fmt.Errorf("read promethues config: %w", err)
	}
	cfg, err := promconfig.Load(string(data))
	if err != nil {
		return nil, fmt.Errorf("load prometheus config: %w", err)
	}
	timeline.startConfig = cfg
	timeline.startState = timeline.getCurrentPolicies()
	return timeline, nil
}

func (t *Timeline) saveAndReload(newConfig string) error {
	if err := ioutil.WriteFile(t.config.PrometheusConfig, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	resp, err := http.Post(t.config.ThanosURL+"/-/reload", "application/json", nil)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	_, err = ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	return err
}

// Reconcile calculates and applies network-polices and scrap configs
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
	var configs []promconfig.ScrapeConfig
	var generatedPolicies []cfnetv1.Policy
	for _, app := range apps {
		// Erase app from startTime if it show up on the timeline
		t.startState = prunePoliciesByDestination(t.startState, app.GUID)
		// Calculate policies and scrape_config sections for app
		policies, endpoints, _ := t.generatePoliciesAndScrapConfigs(app)
		generatedPolicies = append(generatedPolicies, policies...)
		configs = append(configs, endpoints...)
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
	if len(toPrune) > 0 {
		err := t.Networking().RemovePolicies(toPrune)
		if err != nil {
			fmt.Printf("error removing: %v\n", err)
		}
	}
	if len(toAdd) > 0 {
		err := t.Networking().CreatePolicies(toAdd)
		if err != nil {
			fmt.Printf("error creating: %v\n", err)
		}
	}
	t.targets = configs // Refresh the targets list

	// Generate new config
	newCfg, err := promconfig.Load(t.startConfig.String())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	for _, cfg := range configs {
		n := cfg
		newCfg.ScrapeConfigs = append(newCfg.ScrapeConfigs, &n)
	}
	err = t.saveAndReload(newCfg.String())
	if err != nil {
		return err
	}
	fmt.Printf("---config start---\n%s\n---config end---\n", newCfg.String())
	return nil
}

func (t *Timeline) Targets() []promconfig.ScrapeConfig {
	t.Lock()
	targets := t.targets
	defer t.Unlock()
	return targets
}

func (t *Timeline) generatePoliciesAndScrapConfigs(app resources.Application) ([]cfnetv1.Policy, []promconfig.ScrapeConfig, error) {
	var policies []cfnetv1.Policy
	var endpoints []promconfig.ScrapeConfig

	metadata, err := t.metadataRetrieve(appMetadata, app.GUID)
	if err != nil {
		return policies, endpoints, fmt.Errorf("metadataRetrieve: %w", err)
	}
	if port := metadata.Annotations["prometheus.exporter.port"]; port != nil {
		portNumber, err := strconv.Atoi(*port)
		if err != nil {
			return policies, endpoints, err
		}
		policies = append(policies, t.newPolicy(app.GUID, portNumber))
		internalHost, err := t.internalHost(app)
		if err != nil {
			return policies, endpoints, err
		}
		endpoints = append(endpoints, promconfig.ScrapeConfig{
			JobName:         fmt.Sprintf("%s-exporter", app.Name),
			HonorTimestamps: true,
			Scheme:          "http",
			MetricsPath:     "/metrics",
			ServiceDiscoveryConfig: config.ServiceDiscoveryConfig{
				StaticConfigs: []*targetgroup.Group{
					{
						Targets: []model.LabelSet{
							{"__address__": model.LabelValue(fmt.Sprintf("%s:%d", internalHost, portNumber))},
						},
					},
				},
			},
		})
	}
	return policies, endpoints, nil
}

func (t *Timeline) getCurrentPolicies() []cfnetv1.Policy {
	policies, _ := t.Networking().ListPolicies(t.config.ThanosID)
	return policies
}

func (t *Timeline) newPolicy(destination string, port int) cfnetv1.Policy {
	return cfnetv1.Policy{
		Source: cfnetv1.PolicySource{ID: t.config.ThanosID},
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

func (t *Timeline) internalHost(app resources.Application) (string, error) {
	client := t.Session.V2()
	routes, _, err := client.GetApplicationRoutes(app.GUID)
	if err != nil {
		return "", err
	}
	for _, r := range routes {
		if r.DomainGUID == t.config.InternalDomainID {
			return fmt.Sprintf("%s.%s", r.Host, "apps.internal"), nil
		}
	}
	return "", fmt.Errorf("no apps.internal route found")
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
