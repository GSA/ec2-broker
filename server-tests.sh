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
./ec2-broker > tmp/logoutput-result-$$.txt &
started_server=$!
sleep 2 # wait a moment for the server to start

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

sleep 5

aws ec2 describe-instances \
 --filters "Name=tag:cg:ec2broker:brokerInstance,Values=test_instance_$$" > tmp/aws-describe-result-$$.json

reservations_count=$(jq '.Reservations | length' tmp/aws-describe-result-$$.json)
if [ "${reservations_count}" -ne 1 ]
  then
    echo "Found ${reservations_count} reservation(s) after PUT, expected 1"
    exit 1
fi

instances_count=$(jq '.Reservations[0].Instances | length ' tmp/aws-describe-result-$$.json)
if [ "${instances_count}" -ne 1 ]
  then
    echo "Found ${instances_count} instance(s) after PUT, expected 1"
    exit 1
fi

instance_state=$(jq '.Reservations[0].Instances[0].State.Name' tmp/aws-describe-result-$$.json)
if [ "${instance_state}" != '"pending"' -a "${instance_state}" != '"running"' ]; then
  echo "Got state ${instance_state}: should have been 'running' or 'pending'"
  exit 1
fi

aws_instance=$(jq '.Reservations[0].Instances[0].InstanceId' tmp/aws-describe-result-$$.json | sed 's/"//g')

echo "Success"
