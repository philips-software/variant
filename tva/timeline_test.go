package tva_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"variant/tva"

	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/stretchr/testify/assert"
)

var (
	muxCF        *http.ServeMux
	serverCF     *httptest.Server
	muxThanos    *http.ServeMux
	serverThanos *httptest.Server
	muxUAA       *http.ServeMux
	serverUAA    *httptest.Server

	muxLogin    *http.ServeMux
	serverLogin *httptest.Server

	internalDomainID = "xxx"
	thanosID         = "yyy"
	prometheusConfig = "/tmp/prometheus.yml"
)

func setup(t *testing.T) func() {
	muxCF = http.NewServeMux()
	serverCF = httptest.NewServer(muxCF)
	muxThanos = http.NewServeMux()
	serverThanos = httptest.NewServer(muxThanos)
	muxLogin = http.NewServeMux()
	serverLogin = httptest.NewServer(muxLogin)
	muxUAA = http.NewServeMux()
	serverUAA = httptest.NewServer(muxUAA)

	f, err := ioutil.TempFile("", "thanos.yml")
	if !assert.Nil(t, err) {
		return func() {
		}
	}
	_ = ioutil.WriteFile(f.Name(), []byte(`# my global config
global:
  scrape_interval: 15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  external_labels:
    cluster: thanos
    replica: 0

# Alertmanager configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets:
          # - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
  - job_name: 'variant'
    static_configs:
      - targets: ['localhost:1355']`), 0644)
	prometheusConfig = f.Name()

	muxCF.HandleFunc("/v3/apps", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			val := r.URL.Query().Get("label_selector")
			if !assert.Equal(t, "variant.tva/exporter=true", val) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{
  		"pagination": {
    		"total_results": 0,
    		"total_pages": 1,
    		"first": {
     		 	"href": "`+serverCF.URL+`/v3/apps?label_selector=variant.tva%2Fexporter%3Dtrue&page=1&per_page=50"
    		},
    		"last": {
      			"href": "`+serverCF.URL+`/v3/apps?label_selector=variant.tva%2Fexporter%3Dtrue&page=1&per_page=50"
    		},
    		"next": null,
    		"previous": null
  		},
  		"resources": []
	}`)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	muxCF.HandleFunc("/v3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "links": {
    "self": {
      "href": "`+serverCF.URL+`/v3"
    },
    "app_usage_events": {
      "href": "https://`+serverCF.URL+`/v3/app_usage_events"
    },
    "apps": {
      "href": "`+serverCF.URL+`/v3/apps"
    },
    "audit_events": {
      "href": "`+serverCF.URL+`m/v3/audit_events"
    },
    "buildpacks": {
      "href": "`+serverCF.URL+`/v3/buildpacks"
    },
    "builds": {
      "href": "`+serverCF.URL+`m/v3/builds"
    },
    "deployments": {
      "href": "`+serverCF.URL+`/v3/deployments"
    },
    "domains": {
      "href": "`+serverCF.URL+`/v3/domains"
    },
    "droplets": {
      "href": "`+serverCF.URL+`/v3/droplets"
    },
    "environment_variable_groups": {
      "href": "`+serverCF.URL+`/v3/environment_variable_groups"
    },
    "feature_flags": {
      "href": "`+serverCF.URL+`/v3/feature_flags"
    },
    "info": {
      "href": "`+serverCF.URL+`/v3/info"
    },
    "isolation_segments": {
      "href": "`+serverCF.URL+`/v3/isolation_segments"
    },
    "organizations": {
      "href": "`+serverCF.URL+`/v3/organizations"
    },
    "organization_quotas": {
      "href": "`+serverCF.URL+`/v3/organization_quotas"
    },
    "packages": {
      "href": "`+serverCF.URL+`/v3/packages"
    },
    "processes": {
      "href": "`+serverCF.URL+`/v3/processes"
    },
    "resource_matches": {
      "href": "`+serverCF.URL+`/v3/resource_matches"
    },
    "roles": {
      "href": "`+serverCF.URL+`/v3/roles"
    },
    "routes": {
      "href": "`+serverCF.URL+`/v3/routes"
    },
    "security_groups": {
      "href": "`+serverCF.URL+`/v3/security_groups"
    },
    "service_brokers": {
      "href": "`+serverCF.URL+`/v3/service_brokers"
    },
    "service_instances": {
      "href": "`+serverCF.URL+`/v3/service_instances"
    },
    "service_credential_bindings": {
      "href": "`+serverCF.URL+`/v3/service_credential_bindings"
    },
    "service_offerings": {
      "href": "`+serverCF.URL+`/v3/service_offerings"
    },
    "service_plans": {
      "href": "`+serverCF.URL+`/v3/service_plans"
    },
    "service_route_bindings": {
      "href": "`+serverCF.URL+`/v3/service_route_bindings"
    },
    "service_usage_events": {
      "href": "`+serverCF.URL+`/v3/service_usage_events"
    },
    "spaces": {
      "href": "`+serverCF.URL+`/v3/spaces"
    },
    "space_quotas": {
      "href": "`+serverCF.URL+`/v3/space_quotas"
    },
    "stacks": {
      "href": "`+serverCF.URL+`/v3/stacks"
    },
    "tasks": {
      "href": "`+serverCF.URL+`/v3/tasks"
    },
    "users": {
      "href": "`+serverCF.URL+`/v3/users"
    }
  }
}`)
	})

	muxCF.HandleFunc("/v2/info", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "name": "",
  "build": "",
  "support": "",
  "version": 0,
  "description": "",
  "authorization_endpoint": "`+serverLogin.URL+`",
  "token_endpoint": "`+serverUAA.URL+`",
  "min_cli_version": null,
  "min_recommended_cli_version": null,
  "app_ssh_endpoint": "localhost.localdomain:2222",
  "app_ssh_host_key_fingerprint": "3e:d9:f9:02:29:9d:f6:4b:f2:90:fe:4b:05:85:35:8d",
  "app_ssh_oauth_client": "ssh-proxy",
  "doppler_logging_endpoint": "wss://localhost.localdomain:4443",
  "api_version": "2.164.0",
  "osbapi_version": "2.15",
  "routing_endpoint": "`+serverCF.URL+`/routing"
}`)
	})

	muxCF.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{
  "links": {
    "self": {
      "href": "`+serverCF.URL+`"
    },
    "bits_service": null,
    "cloud_controller_v2": {
      "href": "`+serverCF.URL+`/v2",
      "meta": {
        "version": "2.164.0"
      }
    },
    "cloud_controller_v3": {
      "href": "`+serverCF.URL+`/v3",
      "meta": {
        "version": "3.99.0"
      }
    },
    "network_policy_v0": {
      "href": "`+serverCF.URL+`/networking/v0/external"
    },
    "network_policy_v1": {
      "href": "`+serverCF.URL+`/networking/v1/external"
    },
    "login": {
      "href": "`+serverLogin.URL+`"
    },
    "uaa": {
      "href": "`+serverUAA.URL+`"
    },
    "credhub": null,
    "routing": {
      "href": "`+serverCF.URL+`/routing"
    },
    "logging": {
      "href": "wss://localhost.localdomain:4443"
    },
    "log_cache": {
      "href": "https://localhost.localdomain"
    },
    "log_stream": {
      "href": "https://localhost.localdomain"
    },
    "app_ssh": {
      "href": "localhost.localdomain:2222",
      "meta": {
        "host_key_fingerprint": "3e:d9:f9:02:29:9d:f6:4b:f2:90:fe:4b:05:85:35:8d",
        "oauth_client": "ssh-proxy"
      }
    }
  }
}`)
		}
	})

	muxLogin.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	muxLogin.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "access_token": "very_secret",
  "expires_in": 599,
  "id_token": "bogus",
  "jti": "20b8efe4b78a4cb581d0bc70fd522344",
  "refresh_token": "even_more_secret",
  "scope": "clients.read cloud_controller.read password.write cloud_controller.admin_read_only cloud_controller.write openid scim.read uaa.user",
  "token_type": "Bearer"
}`)
	})
	muxLogin.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	muxUAA.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	muxThanos.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	return func() {
		serverCF.Close()
		serverThanos.Close()
		serverLogin.Close()
		serverUAA.Close()
		_ = f.Close()
		_ = syscall.Unlink(f.Name())

	}
}

func TestNewTimeline(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	config := tva.Config{
		Config: clients.Config{
			Endpoint: serverCF.URL,
			User:     "ron",
			Password: "swanson",
		},
		PrometheusConfig: prometheusConfig,
		InternalDomainID: internalDomainID,
		ThanosID:         thanosID,
		ThanosURL:        serverThanos.URL,
	}

	timeline, err := tva.NewTimeline(config,
		tva.WithDebug(true),
		tva.WithFrequency(5),
		tva.WithTenants("default"),
		tva.WithReload(true),
	)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, timeline) {
		return
	}

	done := timeline.Start()

	if !assert.NotNil(t, done) {
		return
	}
	done <- true

	err = timeline.Reconcile()
	if !assert.Nil(t, err) {
		return
	}
}

func TestReconcile(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	config := tva.Config{
		Config: clients.Config{
			Endpoint: serverCF.URL,
			User:     "ron",
			Password: "swanson",
		},
		PrometheusConfig: prometheusConfig,
		InternalDomainID: internalDomainID,
		ThanosID:         thanosID,
		ThanosURL:        serverThanos.URL,
	}

	timeline, err := tva.NewTimeline(config,
		tva.WithDebug(true),
		tva.WithFrequency(5),
		tva.WithTenants("default"),
		tva.WithReload(true),
	)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, timeline) {
		return
	}

	err = timeline.Reconcile()
	if !assert.Nil(t, err) {
		return
	}
}
