package service

import (
	"github.com/pivotal-cf/brokerapi"

	"github.com/GSA/ec2-broker/config"
)

var (
	services []brokerapi.Service
)

/*
GetServiceDescriptions provides a populated arraay of all the exisitng services, built from configuration files.
*/
func GetServiceDescriptions() ([]brokerapi.Service, error) {
	if services != nil {
		return services, nil
	}
	conf := config.GetConfiguration()
	plans := make([]brokerapi.ServicePlan, len(conf.Plans))
	for i := 0; i < len(plans); i++ {
		plans[i] = brokerapi.ServicePlan{
			ID:          conf.Plans[i].ID,
			Name:        conf.Plans[i].Name,
			Description: conf.Plans[i].Description,
		}
	}
	// TODO: Add metadata information in here.
	services := []brokerapi.Service{
		brokerapi.Service{
			ID:            conf.ServiceID,
			Name:          conf.ServiceName,
			Description:   conf.ServiceDescription,
			Bindable:      true,
			PlanUpdatable: false,
			Plans:         plans,
		},
	}
	return services, nil
}
