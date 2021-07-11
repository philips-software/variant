package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
	"timekeeper/tva"

	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

type VCAPApplication struct {
	CfAPI  string `json:"cf_api"`
	Limits struct {
		Fds  int `json:"fds"`
		Mem  int `json:"mem"`
		Disk int `json:"disk"`
	} `json:"limits"`
	ApplicationName    string   `json:"application_name"`
	ApplicationUris    []string `json:"application_uris"`
	Name               string   `json:"name"`
	SpaceName          string   `json:"space_name"`
	SpaceID            string   `json:"space_id"`
	OrganizationID     string   `json:"organization_id"`
	OrganizationName   string   `json:"organization_name"`
	Uris               []string `json:"uris"`
	ProcessID          string   `json:"process_id"`
	ProcessType        string   `json:"process_type"`
	ApplicationID      string   `json:"application_id"`
	Version            string   `json:"version"`
	ApplicationVersion string   `json:"application_version"`
}

const (
	listenPort = 1024 + 116 + 118 + 97
)

func main() {
	var vcapApplication VCAPApplication

	viper.SetEnvPrefix("variant")
	viper.SetDefault("port", listenPort)
	viper.SetDefault("thanos_url", "http://localhost:9090")
	viper.SetDefault("debug", false)
	viper.SetDefault("refresh", 15)
	viper.SetDefault("tenants", "default")
	viper.SetDefault("reload", true)
	viper.AutomaticEnv()

	// Determine thanosID
	thanosID := viper.GetString("thanos_id")
	if thanosID == "" {
		vcap := json.NewDecoder(bytes.NewBufferString(os.Getenv("VCAP_APPLICATION")))
		if err := vcap.Decode(&vcapApplication); err != nil {
			fmt.Printf("not running in CF and no thanosID found in ENV: %v\n", err)
			return
		} else {
			thanosID = vcapApplication.ApplicationID
		}
	}
	fmt.Printf("thanosID: %s\n", thanosID)

	internalDomainID := viper.GetString("internal_domain_id")
	prometheusConfig := viper.GetString("prometheus_config")

	config := tva.Config{
		Config: clients.Config{
			Endpoint: viper.GetString("api_endpoint"),
			User:     viper.GetString("username"),
			Password: viper.GetString("password"),
		},
		PrometheusConfig: prometheusConfig,
		InternalDomainID: internalDomainID,
		ThanosID:         thanosID,
		ThanosURL:        viper.GetString("thanos_url"),
	}

	timeline, err := tva.NewTimeline(config,
		tva.WithDebug(viper.GetBool("debug")),
		tva.WithTenants(viper.GetString("tenants")),
		tva.WithReload(viper.GetBool("reload")),
		tva.WithMetrics(tva.Metrics{
			ScrapeInterval: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "variant_scrape_interval",
				Help: "The last scrape interval duration",
			}),
			DetectedScrapeConfigs: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "variant_scrape_configs_detected",
				Help: "Detected scrape configs",
			}),
			ManagedNetworkPolicies: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "variant_network_policies_managed",
				Help: "The number of network policies being managed by variant",
			}),
			TotalIncursions: promauto.NewCounter(prometheus.CounterOpts{
				Name: "variant_incursions_total",
				Help: "Total number of incursions (scrapes) done by variant so far",
			}),
			ErrorIncursions: promauto.NewCounter(prometheus.CounterOpts{
				Name: "variant_incursions_error",
				Help: "Total number of incursions that went wrong",
			}),
		}),
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	refresh := viper.GetInt("refresh")
	if refresh < 5 {
		fmt.Printf("refresh interval must be at least 5 seconds [%d]\n", refresh)
		return
	}
	done := make(chan bool)

	// Self monitoring
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		_ = http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil)
	}()

	timekeeper(time.Duration(refresh), timeline, done)
}

func timekeeper(tick time.Duration, timeline *tva.Timeline, done <-chan bool) {
	ticker := time.NewTicker(tick * time.Second)
	for {
		select {
		case <-done:
			fmt.Printf("sacred tva is done")
			return
		case <-ticker.C:
			fmt.Printf("reconciling timeline\n")
			err := timeline.Reconcile()
			if err != nil {
				fmt.Printf("error reconciling: %v\n", err)
			}
		}
	}
}
