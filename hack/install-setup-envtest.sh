#!/usr/bin/env bash

# if setup-envtest is not installed, install it

if ! command -v 'setup-envtest' &> /dev/null
then
    echo 'setup-envtest could not be found'
    echo 'installing setup-envtest'
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
else
    echo 'setup-envtest is already installed'
fi