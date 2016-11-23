/*
* A broker that will allow for the creation and deployment of EC2 instances within a limited
* construction - will limit to a certain set of AMIs, VPCs, subnets and security groups.
*
* The provisioning capability will create the EC2 instance
 */
package main

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"

	"github.com/GSA/ec2-broker/broker"
	"github.com/GSA/ec2-broker/config"
)

func main() {
	logger := config.GetLogger()
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	// Bail out if we either cannot read our configuration or connect to AWS
	conf, err := config.LoadConfiguration("config.json")
	if err != nil {
		logger.Fatal("loading-broker", err, nil)
		return
	}

	m, err := broker.NewAWSManager()
	if err != nil {
		logger.Fatal("loading-aws-session", err, nil)
		return
	}

	b, err := broker.New("ec2-broker", m)
	if err != nil {
		logger.Fatal("loading-broker", err, nil)
		return
	}
	// TODO: Remove user/password from configuration file
	handler := brokerapi.New(b, logger, brokerapi.BrokerCredentials{Username: conf.BrokerUsername, Password: conf.BrokerPassword})
	s := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}
	logger.Info("starting-server", lager.Data{"message": fmt.Sprintf("Starting server on port: %s", port)})
	logger.Fatal("listening-error", s.ListenAndServe(), nil)
}
