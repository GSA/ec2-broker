package broker

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"

	"github.com/GSA/ec2-broker/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pivotal-cf/brokerapi"
)

/*
ProvisionAWSInstance will launch and instance and provide the instance ID back.

This will validate the inputs against the configuration to ensure that this can be called. The end result will
be an instance with a tag called brokerInstance = instanceID
*/
func ProvisionAWSInstance(client *ec2.EC2, planID, amiID, securityGroupID, subnetID string, assignPublicIP bool, instanceID string) (string, error) {
	conf := config.GetConfiguration()
	logger := config.GetLogger()
	plan, err := findPlan(conf, planID)
	// Does the request ask for an allowable AMI, security group, subnet and public IP setting?
	if err != nil {
		return "", err
	}
	if !stringIn(amiID, plan.AllowedAMIs) {
		return "", fmt.Errorf("Attempt to start disallowed AMI: %s", amiID)
	}
	if !stringIn(securityGroupID, plan.AllowedSecurityGroups) {
		return "", fmt.Errorf("Attempt to start instance in disallowed security group: %s", securityGroupID)
	}
	if !stringIn(subnetID, plan.AllowedSubnets) {
		return "", fmt.Errorf("Attempt to start instance in disallowed subnet: %s", subnetID)
	}
	if assignPublicIP && !plan.AllowPublicIP {
		return "", errors.New("Attempt to start instance with a public IP while plan does not allow it")
	}

	// Build the instance request, including going a level deeper into the network to allow
	// for us to request a public IP
	instanceType := plan.InstanceType
	nis := ec2.InstanceNetworkInterfaceSpecification{
		AssociatePublicIpAddress: aws.Bool(assignPublicIP),
		DeviceIndex:              aws.Int64(0),
		SubnetId:                 aws.String(subnetID),
		Groups: []*string{
			aws.String(securityGroupID),
		},
	}

	instanceInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(amiID),
		MaxCount:     aws.Int64(1),
		MinCount:     aws.Int64(1),
		InstanceType: aws.String(instanceType),
		KeyName:      aws.String(conf.KeyPairName),
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			&nis,
		},
	}
	reservation, err := client.RunInstances(instanceInput)

	// Fail if we haven't constructed the instance
	if err != nil {
		logger.Error("creating-instance", err, lager.Data{
			"ami_id":            amiID,
			"security_group_id": securityGroupID,
			"subnet_id":         subnetID,
		})
		return "", err
	}

	logger.Info("created-instance", lager.Data{
		"instance_id": reservation.Instances[0].InstanceId,
		"ami_id":      reservation.Instances[0].ImageId,
	})

	err = tagEC2Instance(client, *reservation.Instances[0].InstanceId, map[string]string{
		conf.TagPrefix + "brokerInstance": instanceID,
	})
	if err != nil {
		logger.Error("failed-tagging-instance", err, lager.Data{
			"ami_id":            amiID,
			"security_group_id": securityGroupID,
			"subnet_id":         subnetID,
			"instance_id":       instanceID,
			"aws_instance_id":   reservation.Instances[0].InstanceId,
		})
		// Destroy the instance on failure
		_, innerErr := terminateEC2Instance(client, *reservation.Instances[0].InstanceId)
		if innerErr != nil {
			logger.Error("failed-terminating-instance", err, lager.Data{
				"instance_id": instanceID,
			})
			return "", fmt.Errorf("Failed to terminate instance after failing to tag instance %s (AWS ID: %s): %s (tagging error: %s)", instanceID, *reservation.Instances[0].InstanceId, innerErr, err)
		}
		return "", err
	}

	return *reservation.Instances[0].InstanceId, nil
}

/*
TerminateAWSInstance terminates an EC2 instance given its service instance ID (*not* its AWS Instance ID).
Returns the current status
*/
func TerminateAWSInstance(client *ec2.EC2, instanceID string) (string, error) {
	instance, err := getEC2InstanceByServiceID(client, instanceID)
	if err != nil {
		return "", err
	}
	return terminateEC2Instance(client, *instance.InstanceId)
}

/*
GetAWSInstanceStatus gets the status of an EC2 instance by its service instance ID
*/
func GetAWSInstanceStatus(client *ec2.EC2, instanceID string) (string, error) {
	instance, err := getEC2InstanceByServiceID(client, instanceID)
	if err != nil {
		return "", err
	}
	return *instance.State.Name, nil
}

// Private functions

func findPlan(conf *config.Config, planID string) (*config.PlanConfig, error) {
	for i := 0; i < len(conf.Plans); i++ {
		if conf.Plans[i].ID == planID {
			return &conf.Plans[i], nil
		}
	}
	return nil, fmt.Errorf("Unable to find plan in configuration: %s", planID)
}

func stringIn(s string, arr []string) bool {
	for i := 0; i < len(arr); i++ {
		if s == arr[i] {
			return true
		}
	}
	return false
}

// Tags a given EC2 instance with the passed in map - Instance ID refers to the AWS
// Instance ID, *not* the service instance ID
func tagEC2Instance(client *ec2.EC2, awsInstanceID string, tags map[string]string) error {
	tagStructs := make([]*ec2.Tag, len(tags))
	i := 0
	for k, v := range tags {
		tagStructs[i] = &ec2.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		}
		i++
	}
	tagInput := &ec2.CreateTagsInput{
		Resources: []*string{
			aws.String(awsInstanceID),
		},
		Tags: tagStructs,
	}
	_, err := client.CreateTags(tagInput)
	return err
}

// Terminate an EC2 instance given its awsInstanceID
func terminateEC2Instance(client *ec2.EC2, awsInstanceID string) (string, error) {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			aws.String(awsInstanceID),
		},
	}
	output, err := client.TerminateInstances(input)
	if err != nil {
		return "", err
	}
	return output.TerminatingInstances[0].CurrentState.String(), nil
}

// Extracts an EC2 instance based on a tag named tagPrefix + "brokerInstance" being = serviceID
// This will return brokerapi.ErrInstanceDoesNotExist if no such instance is found
func getEC2InstanceByServiceID(client *ec2.EC2, serviceID string) (*ec2.Instance, error) {
	conf := config.GetConfiguration()
	logger := config.GetLogger()
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + conf.TagPrefix + "brokerInstance"),
				Values: []*string{aws.String(serviceID)},
			},
		},
	}
	output, err := client.DescribeInstances(input)
	if err != nil {
		return nil, err
	}
	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, brokerapi.ErrInstanceDoesNotExist
	}
	if len(output.Reservations) > 1 || len(output.Reservations[0].Instances) > 1 {
		logger.Error("finding-instance", errors.New("Multiple nstances with the same service instance ID"), lager.Data{"brokerID": serviceID})
		return nil, fmt.Errorf("Too many running instances with tag")
	}
	return output.Reservations[0].Instances[0], nil
}
