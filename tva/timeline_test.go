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

	internalDomainID = "409ec4df-d54d-4a93-8428-94999ecb50bc"
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

	muxCF.HandleFunc("/networking/v1/external/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	muxCF.HandleFunc("/v2/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/routes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "total_results": 1,
  "total_pages": 1,
  "prev_url": null,
  "next_url": null,
  "resources": [
    {
      "metadata": {
        "guid": "2dd5eb59-ecb5-4b88-a92d-f9776e7d495d",
        "url": "/v2/routes/2dd5eb59-ecb5-4b88-a92d-f9776e7d495d",
        "created_at": "2021-07-30T09:47:22Z",
        "updated_at": "2021-07-30T09:47:22Z"
      },
      "entity": {
        "host": "ceres",
        "path": "",
        "domain_guid": "409ec4df-d54d-4a93-8428-94999ecb50bc",
        "space_guid": "b6b0855f-df85-41c8-8b6f-52b3a1eabb3d",
        "service_instance_guid": null,
        "port": null,
        "domain_url": "/v2/shared_domains/409ec4df-d54d-4a93-8428-94999ecb50bc",
        "space_url": "/v2/spaces/b6b0855f-df85-41c8-8b6f-52b3a1eabb3d",
        "apps_url": "/v2/routes/2dd5eb59-ecb5-4b88-a92d-f9776e7d495d/apps",
        "route_mappings_url": "/v2/routes/2dd5eb59-ecb5-4b88-a92d-f9776e7d495d/route_mappings"
      }
    }
  ]
}`)
	})

	muxCF.HandleFunc("/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/processes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "pagination": {
    "total_results": 1,
    "total_pages": 1,
    "first": {
      "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/processes?page=1&per_page=50"
    },
    "last": {
      "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/processes?page=1&per_page=50"
    },
    "next": null,
    "previous": null
  },
  "resources": [
    {
      "guid": "9e22fe38-38ce-4af6-b529-44d2853d072f",
      "created_at": "2021-07-30T09:47:23Z",
      "updated_at": "2021-08-09T06:04:23Z",
      "type": "web",
      "command": "hello world",
      "instances": 1,
      "memory_in_mb": 512,
      "disk_in_mb": 1024,
      "health_check": {
        "type": "none",
        "data": {
          "timeout": null,
          "invocation_timeout": null
        }
      },
      "relationships": {
        "app": {
          "data": {
            "guid": "9e22fe38-38ce-4af6-b529-44d2853d072f"
          }
        },
        "revision": {
          "data": {
            "guid": "f2eb0f63-62c1-40b5-86bb-fc3c72119109"
          }
        }
      },
      "metadata": {
        "labels": {},
        "annotations": {}
      },
      "links": {
        "self": {
          "href": "`+serverCF.URL+`/v3/processes/9e22fe38-38ce-4af6-b529-44d2853d072f"
        },
        "scale": {
          "href": "`+serverCF.URL+`/v3/processes/9e22fe38-38ce-4af6-b529-44d2853d072f/actions/scale",
          "method": "POST"
        },
        "app": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f"
        },
        "space": {
          "href": "`+serverCF.URL+`/v3/spaces/b6b0855f-df85-41c8-8b6f-52b3a1eabb3d"
        },
        "stats": {
          "href": "`+serverCF.URL+`/v3/processes/9e22fe38-38ce-4af6-b529-44d2853d072f/stats"
        }
      }
    }
  ]
}`)
	})
	appsHandler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{
  		"pagination": {
    		"total_results": 1,
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
  		"resources": [
			{
      "guid": "9e22fe38-38ce-4af6-b529-44d2853d072f",
      "created_at": "2021-07-30T09:47:23Z",
      "updated_at": "2021-08-09T06:04:23Z",
      "name": "ceres",
      "state": "STARTED",
      "lifecycle": {
        "type": "docker",
        "data": {}
      },
      "relationships": {
        "space": {
          "data": {
            "guid": "b6b0855f-df85-41c8-8b6f-52b3a1eabb3d"
          }
        }
      },
      "metadata": {
        "labels": {
          "variant.tva/exporter": "true",
          "variant.tva/rules": "true"
        },
        "annotations": {
          "prometheus.exporter.path": "/metrics",
          "prometheus.exporter.port": "8080",
		  "prometheus.rules.json": "[{\"annotations\":{\"description\":\"{{ $labels.instance }} waiting http connections is at {{ $value }}\",\"summary\":\"Instance {{ $labels.instance }} has more than 2 waiting connections per minute\"},\"expr\":\"kong_nginx_http_current_connections{state=\\\"waiting\\\"} \\u003e 2\",\"for\":\"1m\",\"labels\":{\"severity\":\"critical\"},\"alert\":\"KongWaitingConnections\"}]",
          "prometheus.rules.blabla.json": "{\"alert\":\"TransactionsHSDPPG\",\"annotations\":{\"description\":\"{{ $labels.instance }}, this is just a test alert\",\"summary\":\"Instance {{ $labels.instance }} has high transaction rate\"},\"expr\":\"irate(pg_stat_database_xact_commit{datname=~\\\"hsdp_pg\\\"}[5m]) \\u003e 8\",\"for\":\"1m\",\"labels\":{\"severity\":\"critical\"}}"
        }
      },
      "links": {
        "self": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f"
        },
        "environment_variables": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/environment_variables"
        },
        "space": {
          "href": "`+serverCF.URL+`/v3/spaces/b6b0855f-df85-41c8-8b6f-52b3a1eabb3d"
        },
        "processes": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/processes"
        },
        "packages": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/packages"
        },
        "current_droplet": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/droplets/current"
        },
        "droplets": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/droplets"
        },
        "tasks": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/tasks"
        },
        "start": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/actions/start",
          "method": "POST"
        },
        "stop": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/actions/stop",
          "method": "POST"
        },
        "revisions": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/revisions"
        },
        "deployed_revisions": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/revisions/deployed"
        },
        "features": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/features"
        }
      }
    }
		]
	}`)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	appHandler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{
      "guid": "9e22fe38-38ce-4af6-b529-44d2853d072f",
      "created_at": "2021-07-30T09:47:23Z",
      "updated_at": "2021-08-09T06:04:23Z",
      "name": "ceres",
      "state": "STARTED",
      "lifecycle": {
        "type": "docker",
        "data": {}
      },
      "relationships": {
        "space": {
          "data": {
            "guid": "b6b0855f-df85-41c8-8b6f-52b3a1eabb3d"
          }
        }
      },
      "metadata": {
        "labels": {
          "variant.tva/exporter": "true",
          "variant.tva/rules": "true"
        },
        "annotations": {
          "prometheus.exporter.path": "/metrics",
          "prometheus.exporter.port": "8080",
		  "prometheus.rules.json": "[{\"annotations\":{\"description\":\"{{ $labels.instance }} waiting http connections is at {{ $value }}\",\"summary\":\"Instance {{ $labels.instance }} has more than 2 waiting connections per minute\"},\"expr\":\"kong_nginx_http_current_connections{state=\\\"waiting\\\"} \\u003e 2\",\"for\":\"1m\",\"labels\":{\"severity\":\"critical\"},\"alert\":\"KongWaitingConnections\"}]",
          "prometheus.rules.1.json": "{\"alert\":\"TransactionsHSDPPG\",\"annotations\":{\"description\":\"{{ $labels.instance }}, this is just a test alert\",\"summary\":\"Instance {{ $labels.instance }} has high transaction rate\"},\"expr\":\"irate(pg_stat_database_xact_commit{datname=~\\\"hsdp_pg\\\"}[5m]) \\u003e 8\",\"for\":\"1m\",\"labels\":{\"severity\":\"critical\"}}"
        }
      },
      "links": {
        "self": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f"
        },
        "environment_variables": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/environment_variables"
        },
        "space": {
          "href": "`+serverCF.URL+`/v3/spaces/b6b0855f-df85-41c8-8b6f-52b3a1eabb3d"
        },
        "processes": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/processes"
        },
        "packages": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/packages"
        },
        "current_droplet": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/droplets/current"
        },
        "droplets": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/droplets"
        },
        "tasks": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/tasks"
        },
        "start": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/actions/start",
          "method": "POST"
        },
        "stop": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/actions/stop",
          "method": "POST"
        },
        "revisions": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/revisions"
        },
        "deployed_revisions": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/revisions/deployed"
        },
        "features": {
          "href": "`+serverCF.URL+`/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f/features"
        }
      }
    }`)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	muxCF.HandleFunc("/v3/apps/9e22fe38-38ce-4af6-b529-44d2853d072f", appHandler)
	muxCF.HandleFunc("/v3/apps", appsHandler)

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
