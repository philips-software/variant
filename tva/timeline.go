package tva

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"sync"
	"time"

	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/percona/promconfig"
	"github.com/percona/promconfig/rules"
	"gopkg.in/yaml.v2"
)

const (
	ExporterLabel                 = "variant.tva/exporter"
	TenantLabel                   = "variant.tva/tenant"
	RulesLabel                    = "variant.tva/rules"
	AnnotationInstanceName        = "prometheus.exporter.instance_name"
	AnnotationInstanceSourceRegex = "prometheus.exporter.instance_source_regex"
	AnnotationExporterPort        = "prometheus.exporter.port"
	AnnotationExporterPath        = "prometheus.exporter.path"
	AnnotationExporterScheme      = "prometheus.exporter.scheme"
	AnnotationExporterJobName     = "prometheus.exporter.job_name"
	AnnotationTargetsPort         = "prometheus.targets.port"
	AnnotationTargetsPath         = "prometheus.targets.path"
	AnnotationRulesJSON           = "prometheus.rules.json"
)

var (
	AnnotationRulesIndexJSONRegex = regexp.MustCompile(`prometheus\.rules\.(\d+|\w+)\.json`)
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
	targets       []promconfig.ScrapeConfig
	Selectors     []string
	defaultTenant bool
	startState    []cfnetv1.Policy
	startConfig   string
	config        Config
	reload        bool
	debug         bool
	metrics       Metrics
	frequency     time.Duration
	expiresAt     time.Time
}

type App struct {
	resources.Application
	OrgName   string
	SpaceName string
}

const twoHours = time.Second * 7200

type ruleFiles map[string][]rules.RuleNode

func NewTimeline(config Config, opts ...OptionFunc) (*Timeline, error) {
	session, err := clients.NewSession(config.Config)
	if err != nil {
		return nil, fmt.Errorf("NewTimeline: %w", err)
	}
	timeline := &Timeline{
		Session:   session,
		expiresAt: time.Now().Add(twoHours),
		Selectors: []string{fmt.Sprintf("%s=true", ExporterLabel)},
		config:    config,
	}
	data, err := ioutil.ReadFile(config.PrometheusConfig)
	if err != nil {
		return nil, fmt.Errorf("read promethues config: %w", err)
	}
	var cfg promconfig.Config
	err = yaml.Unmarshal(data, &cfg)

	if err != nil {
		return nil, fmt.Errorf("load prometheus config: %w", err)
	}
	timeline.startConfig = string(data)
	timeline.startState = timeline.getCurrentPolicies()
	for _, o := range opts {
		if err := o(timeline); err != nil {
			return nil, err
		}
	}
	if timeline.debug {
		fmt.Printf("selectors:\n")
		for _, s := range timeline.Selectors {
			fmt.Printf("%s\n", s)
		}
	}
	return timeline, nil
}

func (t *Timeline) session() (*clients.Session, error) {
	if time.Now().After(t.expiresAt) {
		var err error
		t.Session, err = clients.NewSession(t.config.Config)
		if err != nil {
			t.expiresAt = time.Now()
			return nil, err
		}
		t.expiresAt = time.Now().Add(twoHours)
	}
	return t.Session, nil
}

func (t *Timeline) Start() (done chan bool) {
	// TODO: ensure we only start once
	ticker := time.NewTicker(t.frequency * time.Second)
	doneChan := make(chan bool)
	go func(done <-chan bool) {
		for {
			select {
			case <-done:
				fmt.Printf("sacred tva is done\n")
				return
			case <-ticker.C:
				fmt.Printf("reconciling timeline\n")
				err := t.Reconcile()
				if err != nil {
					fmt.Printf("error reconciling: %v\n", err)
				}
			}
		}
	}(doneChan)

	return doneChan
}

func (t *Timeline) saveAndReload(newConfig string, files ruleFiles) error {
	folder := path.Dir(t.config.PrometheusConfig)

	for n, r := range files {
		content := rules.RuleGroups{
			Groups: []rules.RuleGroup{
				{
					Name:  "VariantGroup",
					Rules: r,
				},
			},
		}
		ruleFile := path.Join(folder, n)
		output, _ := yaml.Marshal(content)
		_ = ioutil.WriteFile(ruleFile, output, 0644)
	}

	if err := ioutil.WriteFile(t.config.PrometheusConfig, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if !t.reload { // Prometheus/Thanos uses inotify
		return nil
	}
	resp, err := http.Post(t.config.ThanosURL+"/-/reload", "application/json", nil)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	_, err = ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return err
}

// Reconcile calculates and applies network-polices and scrap configs
func (t *Timeline) Reconcile() error {
	var foundScrapeConfigs = 0
	var managedNetworkPolicies = 0

	t.Lock()
	defer t.Unlock()

	var startTime = time.Now()
	defer func() {
		duration := time.Since(startTime)
		if t.metrics != nil {
			t.metrics.SetScrapeInterval(float64(duration / time.Millisecond))
			t.metrics.SetManagedNetworkPolicies(float64(managedNetworkPolicies))
			t.metrics.SetDetectedScrapeConfigs(float64(foundScrapeConfigs))
			t.metrics.IncTotalIncursions()
		}
	}()
	session, err := t.session()
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}

	// Retrieve all relevant apps
	apps, _, err := session.V3().GetApplications(ccv3.Query{
		Key:    "label_selector",
		Values: t.Selectors,
	})
	if err != nil {
		return fmt.Errorf("GetApplications: %w", err)
	}
	// Retrieve default apps if applicable
	if len(t.Selectors) > 1 && t.defaultTenant {
		defaultApps, _, err := session.V3().GetApplications(ccv3.Query{
			Key: "label_selector",
			Values: []string{
				t.Selectors[0],
				fmt.Sprintf("!%s", TenantLabel)},
		})
		if err == nil {
			apps = append(apps, defaultApps...)
		}
	}
	// Retrieve apps with rules
	appsWithRules, _, err := session.V3().GetApplications(ccv3.Query{
		Key: "label_selector",
		Values: []string{
			fmt.Sprintf("%s=true", RulesLabel),
		},
	})
	if err != nil {
		appsWithRules = []resources.Application{}
	}

	apps = UniqApps(apps)
	appsWithRules = UniqApps(appsWithRules)

	// Rules
	ruleFilesToSave := make(ruleFiles)
	for _, app := range appsWithRules {
		metadata, err := MetadataRetrieve(session.Raw(), app.GUID)
		if err != nil {
			// TODO: record error here
			continue
		}
		entries, err := ParseRules(metadata)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		ruleFilesToSave[fmt.Sprintf("%s.yml", app.GUID)] = entries
	}

	if t.debug {
		fmt.Printf("found %d matching selectors\n", len(apps))
	}
	// Determine the desired state
	var configs []promconfig.ScrapeConfig
	var generatedPolicies []cfnetv1.Policy
	for _, app := range apps {
		// Erase app from startTime if it shows up on the timeline
		t.startState = PrunePoliciesByDestination(t.startState, app.GUID)
		// Calculate policies and scrape_config sections for app
		policies, endpoints, _ := GeneratePoliciesAndScrapeConfigs(session, t.config.InternalDomainID, t.config.ThanosID, App{Application: app})
		generatedPolicies = append(generatedPolicies, policies...)
		configs = append(configs, endpoints...)
	}
	foundScrapeConfigs = len(configs)
	managedNetworkPolicies = len(generatedPolicies)
	desiredState := UniqPolicies(append(t.startState, generatedPolicies...))
	currentState := t.getCurrentPolicies()
	if t.debug {
		fmt.Printf("desired: %d, current: %d\n", len(desiredState), len(currentState))
	}
	// Calculate add/prune
	var toAdd []cfnetv1.Policy
	for _, p := range desiredState {
		found := false
		for _, q := range currentState {
			if PolicyEqual(p, q) {
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
			if PolicyEqual(p, q) {
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
		for _, p := range toPrune {
			err := session.Networking().RemovePolicies([]cfnetv1.Policy{p})
			if err != nil {
				fmt.Printf("error removing policy [%v]: %v\n", p, err)
				if t.metrics != nil {
					t.metrics.IncErrorIncursions()
				}
			}
		}

	}
	if len(toAdd) > 0 {
		for _, p := range toAdd {
			err := session.Networking().CreatePolicies([]cfnetv1.Policy{p})
			if err != nil {
				fmt.Printf("error creating policy [%v]: %v\n", p, err)
				if t.metrics != nil {
					t.metrics.IncErrorIncursions()
				}
			}
		}

	}
	t.targets = configs // Refresh the targets list

	// Generate new config
	var newCfg promconfig.Config

	err = yaml.Unmarshal([]byte(t.startConfig), &newCfg)
	if err != nil {
		if t.metrics != nil {
			t.metrics.IncErrorIncursions()
		}
		return fmt.Errorf("loading config: %w", err)
	}
	for _, cfg := range configs {
		n := cfg
		newCfg.ScrapeConfigs = append(newCfg.ScrapeConfigs, &n)
	}
	for r := range ruleFilesToSave {
		newCfg.RuleFiles = append(newCfg.RuleFiles, r)
	}

	output, err := yaml.Marshal(newCfg)
	if err != nil {
		if t.metrics != nil {
			t.metrics.IncErrorIncursions()
		}
		return fmt.Errorf("yaml.Marshal: %w", err)
	}
	if t.debug {
		fmt.Printf("---config start---\n%s\n---config end---\n", string(output))
	}

	return t.saveAndReload(string(output), ruleFilesToSave)
}

func (t *Timeline) Targets() []promconfig.ScrapeConfig {
	t.Lock()
	targets := t.targets
	defer t.Unlock()
	return targets
}

func (t *Timeline) getCurrentPolicies() []cfnetv1.Policy {
	allPolicies, _ := t.Networking().ListPolicies(t.config.ThanosID)
	var policies []cfnetv1.Policy
	for _, p := range allPolicies {
		if p.Source.ID == t.config.ThanosID {
			policies = append(policies, p)
		}
	}
	return policies
}

func NewPolicy(source, destination string, port int) cfnetv1.Policy {
	return cfnetv1.Policy{
		Source: cfnetv1.PolicySource{ID: source},
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

func InternalHost(session *clients.Session, internalDomainID string, app App) (string, error) {
	client := session.V2()
	routes, _, err := client.GetApplicationRoutes(app.GUID)
	if err != nil {
		return "", err
	}
	for _, r := range routes {
		if r.DomainGUID == internalDomainID {
			return fmt.Sprintf("%s.%s", r.Host, "apps.internal"), nil
		}
	}
	return "", fmt.Errorf("no apps.internal route found")
}
