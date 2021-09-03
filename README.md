# Variant
A sidecar for [Thanos](https://github.com/philips-labs/terraform-cloudfoundry-thanos) to discover scrape endpoints and rules.
It also takes care of maintaing the network policies so Thanos/Prometheus can do their scraping. 

## Internals
Uses the Cloud foundry API for:
- Discovery of metrics endpoints and scrape targets through CF labels/annotations
- Discovery of rules (alerts, recorders) through CF labels/annotations
- Creates `rule_files_*.yml` containing discovered rules
- Renders `scrape_configs:` and `rule_files:` sections
- Adds/removes CF network policies between Promethues and scrape targets containers
- Writes the `prometheus.yml` config file and triggers Promethues to reload

![variant](resources/variant.png)

## Label and Annotation setup using Terraform

```hcl
resource "cloudfoundry_app" "kong" {
  ...
  labels = {
    "variant.tva/exporter" = true,
    "variant.tva/rules"    = true,
  }
  annotations = {
    "prometheus.exporter.port" = "8001"
    "prometheus.exporter.path" = "/metrics"
    "prometheus.rules.json" = jsonencode([
      {
        alert = "KongWaitingConnections"
        expr = "kong_nginx_http_current_connections{state=\"waiting\"} > 2"
        for = "1m"
        labels = {
          severity = "critical"
        }
        annotations = {
          summary = "Instance {{ $labels.instance }} has more than 2 waiting connections per minute"
          description = "{{ $labels.instance }} waiting http connections is at {{ $value }}"
        }
      }])
  }
}
```

## Labels
Labels control which CF apps `variant` will examine for exporters or rules

| Label | Description |
|-------|-------------|
| `variant.tva/exporter=true` | Variant will examine this app for Metrics exporter endpoints |
| `variant.tva/rules=true` | Variant will look for Prometheus rules in the annotations |

## Annotations
Annotations contain the configurations for metrics and rule definitions

### For exporters

| Annotation | Description | Default       |
|------------|-------------|---------------|
| `prometheus.exporter.port` | The metrics ports to use | `9090` |
| `promethues.exporter.path` | The metrics path to use | `/metrics` |
| `promethues.exporter.instance_name` | The instance name to use (optional) | |
| `promethues.targets.port` | The targets port to use (optional) | |
| `prometheus.targets.path` | The targets path to use (optional) | `/targets` |

### For rules

| Annotation | Description | Default       |
|------------|-------------|---------------|
| `prometheus.rules.json` | JSON string of `[]Rule` | `jsonecode('[]')`
| `prometheus.rules.*.json` | JSON string of a `Rule` object |  |

## License
License is MIT
