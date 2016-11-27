# Limited EC2 Broker for Cloud Foundry

The broker provides limited access to launch EC2 instances.

Via configuration, this broker will provide the ability to launch EC2 instances
from a limited list of AMIs, into a limited list of subnets and security groups.
Right now, that's done via a JSON configuration file launched with the broker.

## Configuration

See [the sample JSON configuration file](config-sample.json) to see format.
This launches a single service with multiple plans. The plans can be varied based on
sizing of machines to launch (Note: this should probably be made more flexible), and
the parameters mentioned above. The broker expects to launch with the standard
environment variables that provide AWS access defined (either via a credentials
file or with the access key variables defined).

The configuration allows you to set
* the service information that will be presented by the CF marketplace,
* the AWS region,
* the broker's username and password,
* the default keypair that will be used when building the EC2 instances,
* the prefix that will be used for tagging the EC2 instances,
* and define the plans

Each plan has a description and allows for creating a list of AMIs, security
groups, and subnets for deployment (See the TODO below in Use.)

## Build

This depends on the [Cloud Foundry brokerapi](https://github.com/pivotal-cf/brokerapi), the
[AWS Go SDK](https://github.com/aws/aws-sdk-go), and [Lager](https://code.cloudfoundry.org/lager)

A good old...

```
$ godep restore
$ go build
```

Should get you to a working `ec2-broker` executable. You'll need to make your own `config.json` file,
using the [config-sample.json](config-sample.json) file as a model.

## Testing

The unit tests require [Ginkgo](https://onsi.github.io/ginkgo/),
[Gomega](https://github.com/onsi/gomega), and [Testify](https://github.com/stretchr/testify).

The usual `go test` will execute the unit tests.

Right now, there is one server-oriented integration test built. The test
requires `jq` and the AWS CLI to be installed to run. The test exercises the
ability for the launched server to connect and provision servers via AWS. The
tests are run from [server-tests.sh](server-tests.sh). It requires a few
environment variables to run - the standard AWS environment variables
that identify a profile or an access key id and secret key value, the
`AWS_REGION` variable, and three variables that identify an accessible
AMI ID, Subnet ID, and Security Group ID. Those values will be interpolated
into the [test configuration file](testdata/config-server-tests.json).

```
$ <AWS parameters> CGN_SN=<subnet id> CGN_SG=<security group id> CGN_AMI=<AMI ID> ./server-tests.sh
```

## Use

This follows the [Cloud Foundry Service Broker V2 API](https://docs.cloudfoundry.org/services/api.html) model
using the Go [brokerapi package](https://github.com/pivotal-cf/brokerapi). It
is an asynchronous broker, so the last operation call is supported.

Requests for provisioning require parameters which identify AMI, subnet, security
groups, and a true/false as to whether the user is requesting a public IP.
Currently, Elastic IP binding and EBS creation isn't supported, but... soon?

(*TODO*: use the tagging namespace more extensively so we can just set up groups, subnets, etc. with
  the right tags and this would no longer depend on configuration file.)

The binding operations will allow you to bring your own public key to a running instance (*not yet implemented*)

## Public domain
This project is in the worldwide [public domain](LICENSE.md).

> This project is in the public domain within the United States, and copyright and related rights in the work worldwide are waived through the [CC0 1.0 Universal public domain dedication](https://creativecommons.org/publicdomain/zero/1.0/).
>
> All contributions to this project will be released under the CC0 dedication. By submitting a pull request, you are agreeing to comply with this waiver of copyright interest.
