#!/bin/bash

# exit on error
set -e

cd cmd/cli

GOOS=linux GOARCH=arm GOARM=7 go build -o ssm2-cli
scp ssm2-cli gavin@192.168.4.61:/home/gavin/ssm2-cli
