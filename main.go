package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"timekeeper/tva"

	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type SDConfig struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

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

func main() {
	var vcapApplication VCAPApplication

	viper.SetEnvPrefix("variant")
	viper.SetDefault("port", "6633")
	viper.SetDefault("thanos_url", "http://localhost:9090")
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

	selectors := []string{"variant.tva/exporter=true"}
	timeline, err := tva.NewTimeline(config, selectors)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	done := make(chan bool)

	go timekeeper(5, timeline, done)

	e := echo.New()
	e.GET("/prometheus", prometheusHandler(timeline))

	port := viper.GetString("port")

	log.Fatal(e.Start(fmt.Sprintf(":%s", port)))
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

func prometheusHandler(timeline *tva.Timeline) echo.HandlerFunc {

	return func(c echo.Context) error {
		var results []SDConfig

		timeline.Lock()
		defer timeline.Unlock()
		return c.JSON(http.StatusOK, results)
	}
}
