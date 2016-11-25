package broker

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager"

	"github.com/GSA/ec2-broker/config"
	"github.com/GSA/ec2-broker/service"
	"github.com/pivotal-cf/brokerapi"
)

/*
EC2Broker stores the baseline information about the EC2 Broker
*/
type EC2Broker struct {
	BrokerName string `json:"broker_name"`
	Manager    InstanceManager
}

/*
ProvisionParameters is the JSON format for the parameters being passed into the provision API call
*/
type ProvisionParameters struct {
	AMIID           string `json:"ami_id"`
	SecurityGroupID string `json:"security_group_id"`
	SubnetID        string `json:"subnet_id"`
	AssignPublicIP  bool   `json:"assign_public_ip"`
}

/*
New creates a new broker and connects to AWS based on the current environment.
*/
func New(name string, m InstanceManager) (*EC2Broker, error) {
	return &EC2Broker{
		BrokerName: name,
		Manager:    m,
	}, nil
}

// Services lists all the broker's services. Currently, there is one service that will provide an EC2 plan
func (b *EC2Broker) Services(context context.Context) []brokerapi.Service {
	services, err := service.GetServiceDescriptions()
	if err != nil {
		return make([]brokerapi.Service, 0)
	}
	return services
}

/*
Provision a new EC2 instance using the parameters provided.

The parameters include the AMI ID to launch, the Subnet to launch it into, and the Security Group to associate it with. The EC2 instance will have a tag called
brokerInstance with a value matching instanceID associated with it. The parameters should be structed as follows:

"parameters: "{
  "ami_id": "<amazon AMI ID>",
  "subnet_id": "<subnet ID>",
  "security_group_id": "<security group ID>"
}

*/
func (b *EC2Broker) Provision(context context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {
	var parameters ProvisionParameters
	logger := config.GetLogger()
	conf := config.GetConfiguration()
	err := json.Unmarshal(details.RawParameters, &parameters)
	if err != nil {
		logger.Info("failed-provision-parse-parameters", lager.Data{"error": err.Error()})
		return brokerapi.ProvisionedServiceSpec{}, brokerapi.ErrRawParamsInvalid
	}
	logger.Info("attempting-provision", lager.Data{
		"plan_id":             details.PlanID,
		"service_instance_id": instanceID,
		"ami_id":              parameters.AMIID,
		"security_group_id":   parameters.SecurityGroupID,
		"subnet_id":           parameters.SubnetID,
		"assign_public_ip":    parameters.AssignPublicIP,
	})
	awsID, err := b.Manager.ProvisionAWSInstance(details.PlanID, parameters.AMIID, parameters.SecurityGroupID, parameters.SubnetID, parameters.AssignPublicIP, instanceID)
	if err != nil {
		logger.Info("failed-provision-creation", lager.Data{"error": err.Error()})
		return brokerapi.ProvisionedServiceSpec{}, err
	}
	logger.Info("created-instance", lager.Data{"aws_instance_id": awsID, "instance_id": instanceID})

	return brokerapi.ProvisionedServiceSpec{
		IsAsync:       true,
		DashboardURL:  conf.DashboardURL,
		OperationData: "provisioning"}, nil
}

/*
Deprovision a managed EC2 instance using the parameters provided.

No parameters are required. The EC2 instance will be terminated. It's the responsibility of the caller to ensure that any stored information is managed before this is called.
*/
func (b *EC2Broker) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	logger := config.GetLogger()
	logger.Info("deprovision", lager.Data{"instanceID": instanceID})
	status, err := b.Manager.TerminateAWSInstance(instanceID)
	if err != nil {
		return brokerapi.DeprovisionServiceSpec{}, err
	}
	return brokerapi.DeprovisionServiceSpec{OperationData: status}, nil
}

/*
Bind the EC2 instance to the caller. This will only succeed after provisioning is complete. The user should call the Bind call with a parameter with the text of a
public SSH key to allow access to the machine. The key text should be directly appendable to an authorized_keys file.

"parmeters": {
  "public_key": "<key text>"
}

After a Bind call is made, another tag will be placed on the EC2 instance called brokerBind with the value being the bindingID
*/
func (b *EC2Broker) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	return brokerapi.Binding{}, errors.New("Bind: function not implemented")
}

/*
Unbind the EC2 instance from the caller.

Currently does nothing, though it should remove the key from the authorized keys....
*/
func (b *EC2Broker) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	return errors.New("Unbind: function not implemented")
}

/*
Update does nothing. The plans are currently not updatable.
*/
func (b *EC2Broker) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, brokerapi.ErrPlanChangeNotSupported
}

/*
LastOperation will look up the current state of an existing provisioned instance from AWS and provide a status back to the user
*/
func (b *EC2Broker) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	status, err := b.Manager.GetAWSInstanceStatus(instanceID)
	if err != nil {
		return brokerapi.LastOperation{}, err
	}
	var state brokerapi.LastOperationState

	switch status {
	case "pending":
		state = brokerapi.InProgress
	case "running":
		state = brokerapi.Succeeded
	default:
		state = brokerapi.Failed // This includes stopped/terminated/stopping states
	}
	return brokerapi.LastOperation{State: state, Description: status}, nil
}
