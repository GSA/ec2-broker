package broker_test

import (
	"context"

	. "github.com/GSA/ec2-broker/broker"

	"github.com/GSA/ec2-broker/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/stretchr/testify/mock"
)

type FakeAWSManager struct {
	mock.Mock
}

func (fm *FakeAWSManager) ProvisionAWSInstance(planID, amiID, securityGroupID, subnetID string, assignPublicIP bool, instanceID string) (string, error) {
	fm.Called(planID, amiID, securityGroupID, subnetID, assignPublicIP, instanceID)
	return "", nil
}

func (fm *FakeAWSManager) TerminateAWSInstance(instanceID string) (string, error) {
	fm.Called(instanceID)
	return "", nil
}

func (fm *FakeAWSManager) GetAWSInstanceStatus(instanceID string) (string, error) {
	fm.Called(instanceID)
	// expect to be passed in i-<AWS state>
	return "", nil
}

var _ = Describe("Broker", func() {
	var (
		m FakeAWSManager
		b *EC2Broker
	)

	BeforeEach(func() {
		config.SetConfiguration(&config.Config{
			DashboardURL:       "http://example.com/dashboard_url",
			Region:             "us-east-1",
			ServiceID:          "service-id",
			ServiceName:        "service-name",
			ServiceDescription: "service-description",
			BrokerUsername:     "broker-user",
			BrokerPassword:     "broker-password",
			KeyPairName:        "key-pair",
			TagPrefix:          "tag-prefix",
			Plans: []config.PlanConfig{
				config.PlanConfig{
					ID:                    "plan-id",
					Name:                  "plan-name",
					Description:           "plan-description",
					InstanceType:          "instance-type",
					AllowedAMIs:           []string{"allowed-ami-1", "allowed-ami-2"},
					AllowedSecurityGroups: []string{"allowed-sg-1", "allowed-sg-2"},
					AllowedSubnets:        []string{"allowed-sn-1", "allowed-sn-2"},
					AllowPublicIP:         true,
				},
			},
		})
		m = FakeAWSManager{}
		b, _ = New("test-broker", &m)
	})

	Describe("provision", func() {
		It("returns the expected catalog", func() {
			services := b.Services(context.Background())
			By("Checking service values")
			Expect(len(services)).To(Equal(1))
			Expect(services[0].ID).To(Equal("service-id"))
			Expect(services[0].Name).To(Equal("service-name"))
			Expect(services[0].Description).To(Equal("service-description"))
			Expect(services[0].PlanUpdatable).To(Equal(false))
			By("Checking plan values")
			Expect(len(services[0].Plans)).To(Equal(1))
			Expect(services[0].Plans[0].ID).To(Equal("plan-id"))
			Expect(services[0].Plans[0].Name).To(Equal("plan-name"))
			Expect(services[0].Plans[0].Description).To(Equal("plan-description"))
		})
	})

	It("fails provision on bad JSON", func() {
		_, err := b.Provision(context.Background(), "instance-1", brokerapi.ProvisionDetails{
			RawParameters: []byte("NOT JSON"),
		}, true)
		Expect(err).To(HaveOccurred())
	})

	It("succeeds provision on valid parameters", func() {
		m.On("ProvisionAWSInstance", "plan-id", "allowed-ami-1", "allowed-sg-1", "allowed-sn-1", true, "instance-1").Return("i-aws-id", nil)
		spec, err := b.Provision(context.Background(), "instance-1",
			brokerapi.ProvisionDetails{
				PlanID:        "plan-id",
				RawParameters: []byte("{ \"ami_id\": \"allowed-ami-1\", \"subnet_id\": \"allowed-sn-1\", \"security_group_id\": \"allowed-sg-1\", \"assign_public_ip\": true }"),
			}, true)
		Expect(err).ToNot(HaveOccurred())
		Expect(spec.OperationData).To(Equal("provisioning"))
		Expect(spec.DashboardURL).To(Equal("http://example.com/dashboard_url"))
		m.AssertExpectations(GinkgoT())
	})

})
