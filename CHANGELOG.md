# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## v0.0.31

- Bugfix: space filtering was broken

## v0.0.30

- Add spaces feature option
- Build with Go 1.18

## v0.0.29

- Add cf_org_name and cf_space_name to static configs (#24)

## v0.0.28

- Fix Docker builds

## v0.0.27

- Dependency upgrade
- First release as philips-software

## v0.0.26

- Fix: be more selective with pruning (#17)

## v0.0.25

- Fix: __address__ format in multi host scraping

## v0.0.24

- Fix: multi host scraping

## v0.0.23

- Fix: refresh session every 2 hour

## v0.0.22

- Support Rules

## v0.0.12

- Fix over-pruning of network policies
- Initial support for instant naming: `prometheus.exporter.instance_name`

## v0.0.11

- Support multiple instances

## v0.0.10

- Add tenant support
- Remove exporter prefix from name
- Make job name configurable: `prometheus.exporter.job_name`
