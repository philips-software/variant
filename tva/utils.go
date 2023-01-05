package tva

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"code.cloudfoundry.org/cfnetworking-cli-api/cfnetworking/cfnetv1"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/percona/promconfig"
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

func ContainsString(haystack []string, needle string) bool {
	for _, a := range haystack {
		if strings.EqualFold(a, needle) {
			return true
		}
	}
	return false
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

	b, err := io.ReadAll(resp.Body)
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

func ParseAutoscaler(metadata Metadata, appGUID string) (*[]Autoscaler, error) {
	var scalers []Autoscaler
	scalerJSON := metadata.Annotations[AnnotationAutoscalerJSON]

	if scalerJSON == nil {
		return nil, fmt.Errorf("missing annotation '%s'", AnnotationAutoscalerJSON)
	}
	err := json.NewDecoder(bytes.NewBufferString(*scalerJSON)).Decode(&scalers)
	if err != nil {
		return nil, fmt.Errorf("decoding scaler JSON: %w", err)
	}
	// Defaults
	for i := 0; i < len(scalers); i++ {
		if scalers[i].Min < 1 {
			scalers[i].Min = 1
		}
		if scalers[i].Max > 50 {
			scalers[i].Max = 50
		}
		if scalers[i].Window == "" {
			scalers[i].Window = "1m"
		}
		if scalers[i].Expression == "" {
			scalers[i].Expression = "query_result > 80"
		}
		if scalers[i].Query == "" {
			scalers[i].Query = `avg(avg_over_time(cpu{guid="{{ guid }}"}[{{ window }}]))`
		}
		scalers[i].GUID = appGUID
	}
	return &scalers, nil
}

func ParseRules(metadata Metadata) ([]rules.RuleNode, error) {
	var foundRules []rules.RuleNode

	rulesJSON := metadata.Annotations[AnnotationRulesJSON]
	if rulesJSON != nil {
		err := json.NewDecoder(bytes.NewBufferString(*rulesJSON)).Decode(&foundRules)
		if err != nil {
			return foundRules, err
		}
	}
	// Add indexed entries as well
	for k, v := range metadata.Annotations {
		if AnnotationRulesIndexJSONRegex.MatchString(k) {
			var rule rules.RuleNode
			err := json.NewDecoder(bytes.NewBufferString(*v)).Decode(&rule)
			if err != nil {
				continue
			}
			foundRules = append(foundRules, rule)
		}
	}

	return foundRules, nil
}

func MetricsEndpointBasicAuthEnabled() bool {
	return viper.GetString("basic_auth_username") != "" && viper.GetString("basic_auth_password") != ""
}

func GeneratePoliciesAndScrapeConfigs(session *clients.Session, internalDomainID, source string, app App) ([]cfnetv1.Policy, []promconfig.ScrapeConfig, error) {
	var policies []cfnetv1.Policy
	var configs []promconfig.ScrapeConfig

	instanceCount := 0
	processes, _, err := session.V3().GetApplicationProcesses(app.GUID)
	if err != nil {
		return policies, configs, err
	}
	for _, p := range processes {
		if p.Instances.IsSet && p.Instances.Value > instanceCount {
			instanceCount = p.Instances.Value
		}
	}
	if instanceCount == 0 {
		return policies, configs, fmt.Errorf("no instances found")
	}
	rawClient := session.Raw()
	metadata, err := MetadataRetrieve(rawClient, app.GUID)
	if err != nil {
		return policies, configs, fmt.Errorf("metadataRetrieve: %w", err)
	}
	portNumber := 9090 // Default
	if port := metadata.Annotations[AnnotationExporterPort]; port != nil {
		portNumber, err = strconv.Atoi(*port)
		if err != nil {
			return policies, configs, err
		}
	}
	scrapePath := "/metrics" // Default
	if exporterPath := metadata.Annotations[AnnotationExporterPath]; exporterPath != nil {
		scrapePath = *exporterPath
	}
	jobName := app.Name // Default
	if name := metadata.Annotations[AnnotationExporterJobName]; name != nil {
		jobName = *name
	}

	appGUID := strings.Split(app.GUID, "-")[0]
	jobName = fmt.Sprintf("%s-%s", jobName, appGUID) // Ensure uniqueness across spaces

	scheme := "http" // Default
	if schema := metadata.Annotations[AnnotationExporterScheme]; schema != nil {
		scheme = *schema
	}

	policies = append(policies, NewPolicy(source, app.GUID, portNumber))
	internalHost, err := InternalHost(session, internalDomainID, app)
	if err != nil {
		return policies, configs, err
	}
	var targets []string
	for count := 0; count < instanceCount; count++ {
		target := fmt.Sprintf("%d.%s:%d", count, internalHost, portNumber)
		targets = append(targets, target)
	}
	scrapeConfig := promconfig.ScrapeConfig{
		JobName: jobName,
		HTTPClientConfig: promconfig.HTTPClientConfig{
			FollowRedirects: true,
		},
		HonorTimestamps: true,
		Scheme:          scheme,
		MetricsPath:     scrapePath,
		ServiceDiscoveryConfig: promconfig.ServiceDiscoveryConfig{
			StaticConfigs: []*promconfig.Group{
				{
					Targets: targets,
					Labels: map[string]string{
						"cf_app_name":   app.Name,
						"cf_space_name": app.SpaceName,
						"cf_org_name":   app.OrgName,
					},
				},
			},
		},
	}
	if scrapeInterval := metadata.Annotations[AnnotationExporterScrapInterval]; scrapeInterval != nil {
		if err := scrapeConfig.ScrapeInterval.Set(*scrapeInterval); err != nil {
			return policies, configs, err
		}
	}
	if MetricsEndpointBasicAuthEnabled() {
		scrapeConfig.HTTPClientConfig = promconfig.HTTPClientConfig{
			BasicAuth: &promconfig.BasicAuth{
				Username: viper.GetString("basic_auth_username"),
				Password: viper.GetString("basic_auth_password"),
			},
			FollowRedirects: true,
		}
	}
	instanceName := ""
	if name := metadata.Annotations[AnnotationInstanceName]; name != nil {
		instanceName = *name
	}
	if instanceName != "" {
		targetRegex := "([^.]*).(.*)" // This match our own target format: ${1} = instanceIndex, ${2} = host:port
		if regex := metadata.Annotations[AnnotationInstanceSourceRegex]; regex != nil {
			targetRegex = *regex
		}
		scrapeConfig.MetricRelabelConfigs = append(scrapeConfig.MetricRelabelConfigs, &promconfig.RelabelConfig{
			TargetLabel:  "instance",
			SourceLabels: []string{"instance"},
			Replacement:  instanceName,
			Action:       "replace",
			Regex:        targetRegex,
		})
	}
	// Multiple host scraping
	if port := metadata.Annotations[AnnotationTargetsPort]; port != nil {
		targetsPort, err := strconv.Atoi(*port)
		if err != nil {
			return policies, configs, err
		}
		targetsPath := "/targets"
		if p := metadata.Annotations[AnnotationTargetsPath]; p != nil {
			targetsPath = *p
		}
		targetsURL := fmt.Sprintf("%s://%s:%d%s", scheme, internalHost, targetsPort, targetsPath)
		policies = append(policies, NewPolicy(source, app.GUID, targetsPort))
		scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs,
			&promconfig.RelabelConfig{
				SourceLabels: []string{"__address__"},
				TargetLabel:  "__param_target",
			},
			&promconfig.RelabelConfig{
				SourceLabels: []string{"__param_target"},
				TargetLabel:  "instance",
			},
			&promconfig.RelabelConfig{
				TargetLabel: "__address__",
				Replacement: fmt.Sprintf("%s:%d", internalHost, portNumber),
			})
		scrapeConfig.ServiceDiscoveryConfig = promconfig.ServiceDiscoveryConfig{
			HTTPSDConfigs: []*promconfig.HTTPSDConfig{
				{URL: targetsURL},
			},
		}
	}
	// Extra relabel config
	if relabelConfigs := metadata.Annotations[AnnotationRelabelConfigs]; relabelConfigs != nil {
		var relabelConfig []*RelabelConfig
		err := json.Unmarshal([]byte(*relabelConfigs), &relabelConfig)
		if err != nil {
			return policies, configs, err
		}
		for _, r := range relabelConfig {
			scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs, r.ToProm())
		}
	}
	configs = append(configs, scrapeConfig)
	return policies, configs, nil
}

func GetMD5Hash(cfg string) string {
	hash := md5.Sum([]byte(cfg))
	return hex.EncodeToString(hash[:])
}
