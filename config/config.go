package config

import (
	"encoding/json"
	"io/ioutil"

	"code.cloudfoundry.org/lager"
)

/*
Config describes the configuration file used to configure this service. It expects that there is only one service, not many
*/
type Config struct {
	DashboardURL       string       `json:"dashboard_url"`
	Region             string       `json:"region"`
	ServiceID          string       `json:"service_id"`
	ServiceName        string       `json:"service_name"`
	ServiceDescription string       `json:"service_description"`
	BrokerUsername     string       `json:"broker_username"`
	BrokerPassword     string       `json:"broker_password"`
	KeyPairName        string       `json:"keypair_name"`
	TagPrefix          string       `json:"tag_prefix"`
	Plans              []PlanConfig `json:"plans"`
}

/*
PlanConfig describes a plan, including the list of allowable subnets, AMIs, and Security groups, and what instance type of EC2 instance will be launched
*/
type PlanConfig struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	InstanceType          string   `json:"instance_type"`
	AllowedAMIs           []string `json:"allowed_amis"`
	AllowedSubnets        []string `json:"allowed_subnets"`
	AllowedSecurityGroups []string `json:"allowed_security_groups"`
	AllowPublicIP         bool     `json:"allow_public_ip"`
}

var (
	config Config
	logger lager.Logger
)

/*
LoadConfiguration pulls from the given filename to load the service configuration from the given JSON file
*/
func LoadConfiguration(filename string) (*Config, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(f, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

/*
GetConfiguration provides the currently configuration to the caller.
*/
func GetConfiguration() *Config {
	return &config
}

/*
GetLogger provides a global logger for this service
*/
func GetLogger() lager.Logger {
	if logger == nil {
		logger = lager.NewLogger("ec2_broker")
	}
	return logger
}
