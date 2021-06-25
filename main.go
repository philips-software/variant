package main

import (
	"fmt"
	"net/http"
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

func main() {
	viper.SetEnvPrefix("")
	viper.SetDefault("port", "8080")
	viper.AutomaticEnv()

	thanosID := viper.GetString("thanos_id")
	config := clients.Config{
		Endpoint: viper.GetString("api_endpoint"),
		User:     viper.GetString("username"),
		Password: viper.GetString("password"),
	}
	selectors := []string{"prometheus.io/exporter=true"}
	timeline, err := tva.NewTimeline(thanosID, selectors, config)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	done := make(chan bool)

	go timekeeper(5, timeline, done)

	e := echo.New()
	e.GET("/prometheus", prometheusHandler(timeline))

	port := viper.GetString("port")

	_ = e.Start(fmt.Sprintf(":%s", port))
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
			_ = timeline.Reconcile()
		}
	}
}

func prometheusHandler(timeline *tva.Timeline) echo.HandlerFunc {
	var results []SDConfig

	return func(c echo.Context) error {
		timeline.Lock()
		defer timeline.Unlock()
		// Do stuff
		return c.JSON(http.StatusOK, results)
	}
}
