#!/usr/bin/env bash

export ORG1CA1_FABRIC_CA_SERVER_CA_KEYFILE=/etc/hyperledger/fabric-ca-server-config/96e12d1fc964e83b1e1c34017f15f50afedbcd002a04d9b74356b81fa42c3697_sk
export ORG2CA1_FABRIC_CA_SERVER_CA_KEYFILE=/etc/hyperledger/fabric-ca-server-config/4adac98b11d6ac1c5dc15ee9cc1ff5ace05e0dd14ece03500633b030f51ebd85_sk
export ORG1TLSCA_FABRIC_CA_SERVER_CA_KEYFILE=/etc/hyperledger/fabric-ca-server-config/84f5003e10eb6f3c231f5776aba819ce3f130edc43d9cc816f0196ed810a63b6_sk

export TEST_CHANGED_ONLY=""
export FABRIC_SDKGO_CODELEVEL_VER="v1.4"
export FABRIC_SDKGO_CODELEVEL_TAG="stable"
export FABRIC_DOCKER_REGISTRY=""
export GO_TESTFLAGS=" -failfast"


cd ../fixtures/dockerenv
docker-compose -f docker-compose-std.yaml -f docker-compose.yaml up --remove-orphans --force-recreate --abort-on-container-exit