# gRPC Server for the cheroAPI service

This repository contains the implementation for the gRPC services, as defined in [cheroapi.proto](https://github.com/luisguve/cheroproto/blob/master/cheroapi.proto) and [userapi.proto](https://github.com/luisguve/cheroproto/blob/master/userapi.proto), using [Bolt](https://github.com/etcd-io/bbolt), an embedded key/value store as the underlying database.

## Table of contents

1. [Installation](#installation)
1. [Services architecture overview](#services-architecture-overview)
1. [Users service](#users-service)
1. [General service](#general-service)
1. [Section service](#section-service)

## Installation

1. run `go get github.com/luisguve/cherosite` and `go get github.com/luisguve/cheroapi`. Then run `go install github.com/luisguve/cherosite/cmd/cherosite` and `go install github.com/luisguve/cheroapi/cmd/...`. The following binaries will be installed in your $GOBIN: `cherosite`, `userapi`, `general` and `contents`. On setup, all of these must be running.
1. You will need to write two .toml files in order to configure the users service and general service and at least one .toml file for a section. Each section should be running on it's own copy of `contents` binary and have it's own .toml file. See userapi.toml, general.toml and section_mylife.toml at the project root for an example.
1. Follow the installation instructions for the http server in the [cherosite project](https://github.com/luisguve/cherosite#Installation).

## Services architecture overview

For the application to work properly, there must be two services running: the users service and the general service, and at least one section service.

### Users service

This service handles everything related to users on a single Bolt database file. 

It's definition can be found at [userapi.proto](https://github.com/luisguve/cheroproto/blob/master/userapi.proto).

### General service

This service handles requests that involve getting contents from multiple sections and multiple users, such as the "/explore" page or the dashboard. It does not store any data, but requests it from the APIs of each section and the API of the users service.

It's definition can be fond at [cheroapi.proto](https://github.com/luisguve/cheroproto/blob/master/cheroapi.proto).

### Section service

This service handles requests of a single section and stores data on a single Bolt database file. It performs one **Quality Assurance** every day automatically, if the variable **schedule_qa** is set to true in the .toml config file for the section.

It's definition can be fond at [cheroapi.proto](https://github.com/luisguve/cheroproto/blob/master/cheroapi.proto).
