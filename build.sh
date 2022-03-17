#!/usr/bin/env bash

cd ./evmwrap
make
cd ..

#file ./evmwrap/host_bridge/libevmwrap.so
# export EVMWRAP=$PWD/evmwrap/host_bridge/libevmwrap.so
# export CGO_LDFLAGS="-L./evmwrap/host_bridge/"

golangci-lint run --build-tags cppbtree

# git clone https://github.com/smartbch/testdata.git
go build ./...
RUN_ALL_EBP_TESTS=NO go test ./...
