#!/bin/bash

tmp=$(mktemp)
yj -yj < config/crd/bases/rabbitmq.com_compositeconsumersets.yaml | jq 'delpaths([.. | paths(scalars)|select(contains(["spec","versions",0,"schema","openAPIV3Schema","properties","spec","properties","consumerPodSpec","description"]))])' | yj -jy > "$tmp"
mv "$tmp" config/crd/bases/rabbitmq.com_compositeconsumersets.yaml