package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"variant/tva"
	"variant/vcap"

	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

const (
	listenPort = 1024 + 116 + 118 + 97
)

type metrics struct {
	ScrapeInterval         prometheus.Gauge
	ManagedNetworkPolicies prometheus.Gauge
	DetectedScrapeConfigs  prometheus.Gauge
	TotalIncursions        prometheus.Counter
	ErrorIncursions        prometheus.Counter
}

func (m metrics) SetScrapeInterval(v float64) {
	m.ScrapeInterval.Set(v)
}

func (m metrics) SetManagedNetworkPolicies(v float64) {
	m.ManagedNetworkPolicies.Set(v)
}

func (m metrics) SetDetectedScrapeConfigs(v float64) {
	m.DetectedScrapeConfigs.Set(v)
}

func (m metrics) IncTotalIncursions() {
	m.TotalIncursions.Inc()
}

func (m metrics) IncErrorIncursions() {
	m.ErrorIncursions.Inc()
}

func main() {
	var vcapApplication vcap.Application

	viper.SetEnvPrefix("variant")
	viper.SetDefault("port", listenPort)
	viper.SetDefault("thanos_url", "http://localhost:9090")
	viper.SetDefault("debug", false)
	viper.SetDefault("refresh", 15)
	viper.SetDefault("tenants", "default")
	viper.SetDefault("spaces", "")
	viper.SetDefault("reload", true)
	viper.AutomaticEnv()

	// Determine thanosID
	thanosID := viper.GetString("thanos_id")
	if thanosID == "" {
		vcapApp := json.NewDecoder(bytes.NewBufferString(os.Getenv("VCAP_APPLICATION")))
		if err := vcapApp.Decode(&vcapApplication); err != nil {
			fmt.Printf("not running in CF and no thanosID found in ENV: %v\n", err)
			return
		} else {
			thanosID = vcapApplication.ApplicationID
		}
	}
	refresh := viper.GetInt("refresh")
	if refresh < 5 {
		fmt.Printf("refresh interval must be at least 5 seconds [%d]\n", refresh)
		return
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
	metrics := metrics{
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
	}

	timeline, err := tva.NewTimeline(config,
		tva.WithDebug(viper.GetBool("debug")),
		tva.WithFrequency(refresh),
		tva.WithTenants(viper.GetString("tenants")),
		tva.WithSpaces(viper.GetString("spaces")),
		tva.WithReload(viper.GetBool("reload")),
		tva.WithMetrics(metrics),
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	done := timeline.Start()

	// Self monitoring
	http.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil)

	done <- true
}
