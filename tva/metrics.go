package tva

type Metrics interface {
	SetScrapeInterval(float64)
	SetManagedNetworkPolicies(float64)
	SetDetectedScrapeConfigs(float64)
	IncTotalIncursions()
	IncErrorIncursions()
}
