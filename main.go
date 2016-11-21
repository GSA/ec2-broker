/*
A broker that will allow for the creation and deployment of EC2 instances within a limited
construction - will limit to a certain set of AMIs, VPCs, subnets and security groups.

The provision
*/
package main

import (
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	logger := lager.NewLogger("ec2_broker")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	handler := brokerapi.New(nil, logger, brokerapi.BrokerCredentials{Username: "user", Password: "pass"})
	s := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}
	logger.Fatal("http-server", s.ListenAndServe(), nil)
}
