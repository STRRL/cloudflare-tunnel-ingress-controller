#!/usr/bin/env bash

# if controller-gen is not installed, install it
set -euo pipefail
if ! command -v 'controller-gen' &> /dev/null
then
    echo 'controller-gen could not be found'
    echo 'installing controller-gen'
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
fi
