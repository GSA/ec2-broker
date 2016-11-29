#!/bin/bash

# This expects that the AWS credential variables and AWS_REGION is set.
# PORT defaults to 8000 # are set so they can execute.

# It also assumes the availability of jq and the aws CLI

# Additionally, CGN_AMI should list an allowed AMI, CGN_SG an allowed security
# group, and CGN_SN an allowed subnet. The corresponding values will be
# replaced in testdata/config-server-tests.json

set -e
set -u

started_server=0
server_port=${PORT-8000}
aws_instance=
instance_state=

# Setup

echo "Setting up configuration"

if [ ! -f testdata/config-server-tests.json ]; then
    echo "Was not able to find testdata/config-server-tests.json"
    exit 1
fi

if [ -f config.json ]; then
  mv config.json tmp/config-$$.json
fi

if [ ! -d tmp ]; then
  mkdir tmp
fi

# Gets the information about the expected test instance from AWS
# Populates aws_instance with the AWS Instance ID
# and instance_state with the AWS state and creates tmp/aws-describe-result-$$.json
lookup_aws_instance ()
{
  aws ec2 describe-instances \
   --filters "Name=tag:cg:ec2broker:brokerInstance,Values=test_instance_$$" > tmp/aws-describe-result-$$.json

  local reservations_count=$(jq '.Reservations | length' tmp/aws-describe-result-$$.json)
  if [ "${reservations_count}" -ne 1 ]
    then
      echo "Found ${reservations_count} reservation(s) after PUT, expected 1"
      exit 1
  fi

  local instances_count=$(jq '.Reservations[0].Instances | length ' tmp/aws-describe-result-$$.json)
  if [ "${instances_count}" -ne 1 ]
    then
      echo "Found ${instances_count} instance(s) after PUT, expected 1"
      exit 1
  fi

  instance_state=$(jq '.Reservations[0].Instances[0].State.Name' tmp/aws-describe-result-$$.json | sed 's/"//g')

  aws_instance=$(jq '.Reservations[0].Instances[0].InstanceId' tmp/aws-describe-result-$$.json | sed 's/"//g')
}

# Kill the web server and cleanup files
teardown() {
  if [ -f config-$$.json ]; then
    rm config.json
    mv tmp/config-$$.json config.json
  fi
  # rm tmp/*-$$.json
  if [ ${started_server} -ne 0 ]; then
    kill ${started_server}
  fi
  # This will try to shut down any instance that remains
  if [ -n "${aws_instance:+x}" ]; then
    aws ec2 terminate-instances --instance-ids ${aws_instance} > /dev/null
  fi
}

trap teardown EXIT



cat testdata/config-server-tests.json \
| sed "s/-CGN_AMI-/${CGN_AMI}/g" \
| sed "s/-CGN_SG-/${CGN_SG}/g" \
| sed "s/-CGN_SN-/${CGN_SN}/g" > config.json

# Spawn the server
echo "Starting server"

./ec2-broker > tmp/logoutput-result-$$.txt &
started_server=$!
sleep 2 # wait a moment for the server to start

echo "Testing catalog endpoint"

curl --silent --user buser:bpassword http://localhost:${server_port}/v2/catalog > tmp/catalog-result-$$.json

if [ $? -ne 0 ]; then
  echo "Curl failed with error $?"
  exit 1
fi

# Make sure there's only one service

service_count=$(jq '.services | length' tmp/catalog-result-$$.json)
if [ $? -ne 0 ]; then
  echo "service_count: jq failed with error $?"
  exit 1
fi

if [ "${service_count}" -ne 1 ]
  then
    echo "Found ${service_count} service(s) in catalog, expected 1"
    exit 1
fi

# Make sure there are two plans

plan_count=$(jq '.services[0].plans | length' tmp/catalog-result-$$.json)
if [ $? -ne 0 ]; then
  echo "plan_count: jq failed with error $?"
  exit 1
fi

if [ "${plan_count}" -ne 2 ]
  then
    echo "Found ${plan_count} plans(s) in catalog, expected 2"
    exit 1
fi

# Provision a new server, query against the tag to find out if it was started

echo "Testing provisioning endpoint"

cat testdata/provision-test.json \
| sed "s/-CGN_AMI-/${CGN_AMI}/g" \
| sed "s/-CGN_SG-/${CGN_SG}/g" \
| sed "s/-CGN_SN-/${CGN_SN}/g" > tmp/provision-test-$$.json

curl --silent --user buser:bpassword -XPUT --data "@tmp/provision-test-$$.json" \
 http://localhost:${server_port}/v2/service_instances/test_instance_$$ > tmp/service-create-result-$$.json

if [ $? -ne 0 ]; then
  echo "provision: curl failed on provision"
  exit 1
fi

lookup_aws_instance

if [ "${instance_state}" != "pending" -a "${instance_state}" != "running" ]; then
  echo "Got state ${instance_state}: should have been 'running' or 'pending'"
  exit 1
fi

sleep 2

# Use last operation to poll until we get a status we want....

echo "Polling last operation endpoint"

while true; do
  echo "  Making call to last operation endpoint"
  curl --silent --user buser:bpassword "http://localhost:${server_port}/v2/service_instances/test_instance_$$/last_operation?service_id=server-test-service-id-1&plan_id=server-test-plan-id-1&operation=p_test_instance_$$" > tmp/lo-result-$$.json
  lo_state=$(jq '.state' tmp/lo-result-$$.json | sed 's/"//g')
  echo "  Got ${lo_state}"
  if [ "${lo_state}" == "in progress" ]; then
    sleep 10
  elif [ "${lo_state}" == "failed" ]; then
    echo "Last Operation returned failed"
    exit 1
  elif [ "${lo_state}" == "succeeded" ]; then
    break
  else
    echo "provision: Got an unexpected state on last operation: ${lo_state}"
    exit 1
  fi
done

echo "Finished polling last operation"

lookup_aws_instance

if [ "${instance_state}" != "running" ]; then
  echo "After waiting for last operation to return success, instance should have been running"
  exit 1
fi

# Deprovision the started instance

echo "Testing deprovisioning endpoint"

curl --silent --user buser:bpassword -XDELETE --data "@tmp/provision-test-$$.json" \
 http://localhost:${server_port}/v2/service_instances/test_instance_$$ > tmp/service-delete-result-$$.json

if [ $? -ne 0 ]; then
  echo "deprovision: curl failed on deprovision"
  exit 1
fi

sleep 5

aws_instance_copy=${aws_instance}

lookup_aws_instance

if [ "${instance_state}" != "shutting-down" \
    -a "${instance_state}" != "terminated" \
    -a "${instance_state}" != "stopping" \
    -a "${instance_state}" != "stopped" ]; then
  echo "Got state ${instance_state}: should have been 'shutting-down', 'stopping', 'stopped', or 'terminated'"
  exit 1
fi

if [ "${aws_instance}" != "${aws_instance_copy}" ]; then
  echo "Was not able to match expected instance ${aws_instance} != ${aws_instance_copy}"
  exit 1
fi

# Use last operation to poll until we get a status we want....
echo "Polling last operation endpoint after deprovision"

while true ; do
  echo "  Making call to last operation endpoint"
  curl --silent --user buser:bpassword "http://localhost:${server_port}/v2/service_instances/test_instance_$$/last_operation?service_id=server-test-service-id-1&plan_id=server-test-plan-id-1&operation=d_test_instance_$$" > tmp/lo-result-$$.json
  lo_state=$(jq '.state' tmp/lo-result-$$.json | sed 's/"//g')
  echo "  Got ${lo_state}"
  if [ "${lo_state}" == "in progress" ]; then
    sleep 10
  elif [ "${lo_state}" == "failed" ]; then
    echo "Last Operation returned failed"
    exit 1
  elif [ "${lo_state}" == "succeeded" ]; then
    break
  else
    echo "deprovision: Got an unexpected state on last operation: ${lo_state}"
    exit 1
  fi
done

echo "Completed deprovision"

lookup_aws_instance

if [ "${instance_state}" != "stopped" -a "${instance_state}" != "terminated" ]; then
  echo "After waiting for last operation to return succees, instance should have been stopped or terminated: ${instance_state}"
  exit 1
fi

# null this out now that we are deleting the instance, so cleanup doesn't try to
# terminate it
aws_instance=

echo "Success"
