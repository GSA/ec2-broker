package broker_test

import (
	"context"
	"errors"

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
	args := fm.Called(planID, amiID, securityGroupID, subnetID, assignPublicIP, instanceID)
	return args.String(0), args.Error(1)
}

func (fm *FakeAWSManager) TerminateAWSInstance(instanceID string) (string, error) {
	args := fm.Called(instanceID)
	return args.String(0), args.Error(1)
}

func (fm *FakeAWSManager) GetAWSInstanceStatus(instanceID string) (string, error) {
	args := fm.Called(instanceID)
	return args.String(0), args.Error(1)
}

var _ = Describe("Broker", func() {
	var (
		m FakeAWSManager
		b *EC2Broker
	)
	// config.GetLogger().RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

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

	Describe("services", func() {
		It("returns the expected catalog", func() {
			services := b.Services(context.Background())
			By("Checking service values")
			Expect(services).To(HaveLen(1))
			Expect(services[0].ID).To(Equal("service-id"))
			Expect(services[0].Name).To(Equal("service-name"))
			Expect(services[0].Description).To(Equal("service-description"))
			Expect(services[0].PlanUpdatable).To(Equal(false))
			By("Checking plan values")
			Expect(services[0].Plans).To(HaveLen(1))
			Expect(services[0].Plans[0].ID).To(Equal("plan-id"))
			Expect(services[0].Plans[0].Name).To(Equal("plan-name"))
			Expect(services[0].Plans[0].Description).To(Equal("plan-description"))
		})
	})

	Describe("provision", func() {
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
			m.AssertExpectations(GinkgoT())
		})

		It("fails provision on provision error return", func() {
			m.On("ProvisionAWSInstance", "plan-id", "allowed-ami-2", "allowed-sg-1", "allowed-sn-1", true, "instance-1").Return("", errors.New("AWS failure"))
			_, err := b.Provision(context.Background(), "instance-1",
				brokerapi.ProvisionDetails{
					PlanID:        "plan-id",
					RawParameters: []byte("{ \"ami_id\": \"allowed-ami-2\", \"subnet_id\": \"allowed-sn-1\", \"security_group_id\": \"allowed-sg-1\", \"assign_public_ip\": true }"),
				}, true)
			Expect(err).To(HaveOccurred())
			m.AssertExpectations(GinkgoT())
		})

	})

	Describe("deprovision", func() {
		It("succeeds on termination for known instances", func() {
			m.On("TerminateAWSInstance", "instance-1").Return("stopping", nil)
			status, err := b.Deprovision(context.Background(), "instance-1", brokerapi.DeprovisionDetails{}, true)
			Expect(err).To(Not(HaveOccurred()))
			Expect(status.OperationData).To(Equal("stopping"))
			Expect(status.IsAsync).To(Equal(true))
			m.AssertExpectations(GinkgoT())
		})

		It("fails termination on AWS error", func() {
			m.On("TerminateAWSInstance", "unknown").Return("", errors.New("AWS Error"))
			status, err := b.Deprovision(context.Background(), "unknown", brokerapi.DeprovisionDetails{}, true)
			Expect(err).To(HaveOccurred())
			Expect(status.OperationData).To(Equal(""))
			m.AssertExpectations(GinkgoT())
		})
	})

	Describe("binding", func() {

		It("fails binding calls", func() {
			_, err := b.Bind(context.Background(), "instance-1", "binding-1", brokerapi.BindDetails{})
			Expect(err).To(HaveOccurred())

			err = b.Unbind(context.Background(), "instance-1", "binding-1", brokerapi.UnbindDetails{})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("update", func() {
		It("fails update calls", func() {
			_, err := b.Update(context.Background(), "instance-1", brokerapi.UpdateDetails{}, true)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(brokerapi.ErrPlanChangeNotSupported))
		})
	})

})
