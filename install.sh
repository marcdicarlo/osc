#!/usr/bin/env bash
# build and install the osc cli

# build the osc cli
go build -o osc cmd/osc/main.go

# install the osc cli
if [ $? -ne 0 ]; then
    echo "Failed to build the osc cli"
    exit 1
fi

sudo cp osc /usr/local/bin/osc

if [ $? -ne 0 ]; then
    echo "Failed to install the osc cli"
    exit 1
fi

echo "osc cli installed successfully"