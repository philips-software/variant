package tva_test

import (
	"testing"
	"variant/tva"

	"code.cloudfoundry.org/cli/resources"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper"
	"github.com/stretchr/testify/assert"
)

func TestParseRules(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	appGUID := "9e22fe38-38ce-4af6-b529-44d2853d072f"

	session, err := clients.NewSession(clients.Config{
		Endpoint: serverCF.URL,
		User:     "ron",
		Password: "swanson",
	})
	if !assert.Nil(t, err) {
		return
	}
	metadata, err := tva.MetadataRetrieve(session.Raw(), appGUID)
	if !assert.Nil(t, err) {
		return
	}
	parsedRules, err := tva.ParseRules(metadata)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, 2, len(parsedRules)) {
		return
	}
	assert.Equal(t, "1m", parsedRules[0].For)
	assert.Equal(t, "KongWaitingConnections", parsedRules[0].Alert)
	assert.Equal(t, "1m", parsedRules[1].For)
	assert.Equal(t, "TransactionsHSDPPG", parsedRules[1].Alert)
}

func TestGeneratePoliciesAndScrapeConfigs(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	session, err := clients.NewSession(clients.Config{
		Endpoint: serverCF.URL,
		User:     "ron",
		Password: "swanson",
	})
	if !assert.Nil(t, err) {
		return
	}
	appGUID := "9e22fe38-38ce-4af6-b529-44d2853d072f"

	app := tva.App{
		Application: resources.Application{
			GUID:  appGUID,
			Name:  "ceres",
			State: "STARTED",
		},
	}
	policies, configs, err := tva.GeneratePoliciesAndScrapeConfigs(session, internalDomainID, thanosID, app)
	assert.Nil(t, err)
	assert.Len(t, policies, 1)
	assert.Len(t, configs, 1)

}
