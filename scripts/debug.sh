#!/bin/bash

source <(trdl use 1.2 alpha)
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d' --docker-compose-options='-f docker-compose-debug.yml'
