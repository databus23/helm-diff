# Helm Diff Plugin
[![Go Report Card](https://goreportcard.com/badge/github.com/databus23/helm-diff)](https://goreportcard.com/report/github.com/databus23/helm-diff)
[![GoDoc](https://godoc.org/github.com/databus23/helm-diff?status.svg)](https://godoc.org/github.com/databus23/helm-diff)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/databus23/helm-diff/blob/master/LICENSE)

This is a Helm plugin giving you a preview of what a `helm upgrade` would change.
It basically generates a diff between the latest deployed version of a release
and a `helm upgrade --debug --dry-run`. This can also be used to compare two
revisions/versions of your helm release.

<a href="https://asciinema.org/a/105326" target="_blank"><img src="https://asciinema.org/a/105326.png" /></a>

## Install

### Using Helm plugin manager (> 2.3.x)

```shell
helm plugin install https://github.com/databus23/helm-diff
```

### Pre Helm 2.3.0 Installation
Pick a release tarball from the [releases](https://github.com/databus23/helm-diff/releases) page.

Unpack the tarball in your helm plugins directory (`$(helm home)/plugins`).

E.g.
```
curl -L $TARBALL_URL | tar -C $(helm home)/plugins -xzv
```

### From Source
#### Prerequisites
 - GoLang `>= 1.17`

Make sure you do not have a version of `helm-diff` installed. You can remove it by running `helm plugin uninstall diff`

#### Installation Steps
The first step is to download the repository and enter the directory. You can do this via `git clone` or downloading and extracting the release. If you clone via git, remember to checkout the latest tag for the latest release.

Next, depending on which helm version you have, install the plugin into helm.

##### Helm 2
```bash
make install
```

##### Helm 3
```bash
make install/helm3
```


## Usage

```
The Helm Diff Plugin

* Shows a diff explaining what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be
  used visualize what changes a helm upgrade will perform.

* Shows a diff explaining what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used visualize what changes
  were made during revision change.

* Shows a diff explaining what a helm rollback would change:
    This fetches the currently deployed version of a release
  and compares it to the previously deployed version of the release, that you
  want to rollback. This can be used visualize what changes a
  helm rollback will perform.

Usage:
  diff [flags]
  diff [command]

Available Commands:
  release     Shows diff between release's manifests
  revision    Shows diff between revision's manifests
  rollback    Show a diff explaining what a helm rollback could perform
  upgrade     Show a diff explaining what a helm upgrade would change.
  version     Show version of the helm diff plugin

Flags:
      --allow-unreleased             enables diffing of releases that are not yet deployed via Helm
  -a, --api-versions stringArray     Kubernetes api versions used for Capabilities.APIVersions
      --color                        color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
  -C, --context int                  output NUM lines of context around changes (default -1)
      --detailed-exitcode            return a non-zero exit code when there are changes
      --devel                        use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --disable-openapi-validation   disables rendered templates validation against the Kubernetes OpenAPI Schema
      --disable-validation           disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install
      --dry-run                      disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation
  -h, --help                         help for diff
      --include-tests                enable the diffing of the helm test hooks
      --install                      enables diffing of releases that are not yet deployed via Helm (equivalent to --allow-unreleased, added to match "helm upgrade --install" command
      --kube-version string          Kubernetes version used for Capabilities.KubeVersion
      --kubeconfig string            This flag is ignored, to allow passing of this top level flag to helm
      --no-color                     remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-hooks                     disable diffing of hooks
      --normalize-manifests          normalize manifests before running diff to exclude style differences from the output
      --output string                Possible values: diff, simple, json, template. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --post-renderer string         the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path
      --repo string                  specify the chart repository url to locate the requested chart
      --reset-values                 reset the values to the ones built into the chart and merge in any new values
      --reuse-values                 reuse the last release's values and merge in any new values. If '--reset-values' is specified, this is ignored
      --set stringArray              set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray         set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-string stringArray       set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --show-secrets                 do not redact secret values in the output
      --strip-trailing-cr            strip trailing carriage return on input
      --suppress stringArray         allows suppression of the values listed in the diff output
  -q, --suppress-secrets             suppress secrets in the output
      --three-way-merge              use three-way-merge to compute patch and generate diff output
  -f, --values valueFiles            specify values in a YAML file (can specify multiple) (default [])
      --version string               specify the exact chart version to use. If this is not specified, the latest version is used

Additional help topics:
  diff

Use "diff [command] --help" for more information about a command.
```

## Commands:

### upgrade:

```
$ helm diff upgrade -h
Show a diff explaining what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a chart plus values.
This can be used visualize what changes a helm upgrade will
perform.

Usage:
  diff upgrade [flags] [RELEASE] [CHART]

Examples:
  helm diff upgrade my-release stable/postgresql --values values.yaml

  # Set HELM_DIFF_IGNORE_UNKNOWN_FLAGS=true to ignore unknown flags
  # It's useful when you're using `helm-diff` in a `helm upgrade` wrapper.
  # See https://github.com/databus23/helm-diff/issues/278 for more information.
  HELM_DIFF_IGNORE_UNKNOWN_FLAGS=true helm diff upgrade my-release stable/postgres --wait

  # Set HELM_DIFF_USE_UPGRADE_DRY_RUN=true to
  # use `helm upgrade --dry-run` instead of `helm template` to render manifests from the chart.
  # See https://github.com/databus23/helm-diff/issues/253 for more information.
  HELM_DIFF_USE_UPGRADE_DRY_RUN=true helm diff upgrade my-release datadog/datadog

  # Set HELM_DIFF_THREE_WAY_MERGE=true to
  # enable the three-way-merge on diff.
  # This is equivalent to specifying the --three-way-merge flag.
  # Read the flag usage below for more information on --three-way-merge.
  HELM_DIFF_THREE_WAY_MERGE=true helm diff upgrade my-release datadog/datadog

  # Set HELM_DIFF_NORMALIZE_MANIFESTS=true to",
	# normalize the yaml file content when using helm diff.",
	# This is equivalent to specifying the --normalize-manifests flag.",
	# Read the flag usage below for more information on --normalize-manifests.",
	HELM_DIFF_NORMALIZE_MANIFESTS=true helm diff upgrade my-release datadog/datadog",

Flags:
      --allow-unreleased             enables diffing of releases that are not yet deployed via Helm
  -a, --api-versions stringArray     Kubernetes api versions used for Capabilities.APIVersions
  -C, --context int                  output NUM lines of context around changes (default -1)
      --detailed-exitcode            return a non-zero exit code when there are changes
      --devel                        use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --disable-openapi-validation   disables rendered templates validation against the Kubernetes OpenAPI Schema
      --disable-validation           disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install
      --dry-run                      disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation
  -h, --help                         help for upgrade
      --include-tests                enable the diffing of the helm test hooks
      --install                      enables diffing of releases that are not yet deployed via Helm (equivalent to --allow-unreleased, added to match "helm upgrade --install" command
      --kube-version string          Kubernetes version used for Capabilities.KubeVersion
      --kubeconfig string            This flag is ignored, to allow passing of this top level flag to helm
      --no-hooks                     disable diffing of hooks
      --normalize-manifests          normalize manifests before running diff to exclude style differences from the output, or use HELM_DIFF_NORMALIZE_MANIFESTS=true.
      --output string                Possible values: diff, simple, json, template. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --post-renderer string         the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path
      --repo string                  specify the chart repository url to locate the requested chart
      --reset-values                 reset the values to the ones built into the chart and merge in any new values
      --reuse-values                 reuse the last release's values and merge in any new values. If '--reset-values' is specified, this is ignored
      --set stringArray              set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray         set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-string stringArray       set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --show-secrets                 do not redact secret values in the output
      --strip-trailing-cr            strip trailing carriage return on input
      --suppress stringArray         allows suppression of the values listed in the diff output
  -q, --suppress-secrets             suppress secrets in the output
      --three-way-merge              use three-way-merge to compute patch and generate diff output, or use HELM_DIFF_THREE_WAY_MERGE=true.
  -f, --values valueFiles            specify values in a YAML file (can specify multiple) (default [])
      --version string               specify the exact chart version to use. If this is not specified, the latest version is used

Global Flags:
      --no-color   remove colors from the output
```

### release:

```
$ helm diff release -h

This command compares the manifests details of a different release created from the same chart

It can be used to compare the manifests of

 - release1 with release2
	$ helm diff release [flags] release1 release2
   Example:
	$ helm diff release my-prod my-stage

Usage:
  diff release [flags] RELEASE release1 [release2]

Flags:
  -C, --context int            output NUM lines of context around changes (default -1)
  -h, --help                   help for release
      --home string            location of your Helm config. Overrides $HELM_HOME (default "/home/aananth/.helm")
      --include-tests          enable the diffing of the helm test hooks
      --strip-trailing-cr      strip trailing carriage return on input
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output
      --tls                    enable TLS for request
      --tls-ca-cert string     path to TLS CA certificate file (default "$HELM_HOME/ca.pem")
      --tls-cert string        path to TLS certificate file (default "$HELM_HOME/cert.pem")
      --tls-hostname string    the server name used to verify the hostname on the returned certificates from the server
      --tls-key string         path to TLS key file (default "$HELM_HOME/key.pem")
      --tls-verify             enable TLS for request and verify remote

Global Flags:
      --no-color   remove colors from the output
```

### revision:

```
$ helm diff revision -h

This command compares the manifest details of a named release.

It can be used to compare the manifests of

 - latest REVISION with specified REVISION
	$ helm diff revision [flags] RELEASE REVISION1
   Example:
	$ helm diff revision my-release 2

 - REVISION1 with REVISION2
	$ helm diff revision [flags] RELEASE REVISION1 REVISION2
   Example:
	$ helm diff revision my-release 2 3

Usage:
  diff revision [flags] RELEASE REVISION1 [REVISION2]

Flags:
  -h, --help                   help for revision
      --strip-trailing-cr      strip trailing carriage return on input
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output

Global Flags:
      --no-color   remove colors from the output
```

### rollback:

```
$ helm diff rollback -h

This command compares the latest manifest details of a named release
with specific revision values to rollback.

It forecasts/visualizes changes, that a helm rollback could perform.

Usage:
  diff rollback [flags] [RELEASE] [REVISION]

Examples:
  helm diff rollback my-release 2

Flags:
  -h, --help                   help for rollback
      --strip-trailing-cr      strip trailing carriage return on input
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output

Global Flags:
      --no-color   remove colors from the output
```

## Build

Clone the repository into your `$GOPATH` and then build it.

```
$ mkdir -p $GOPATH/src/github.com/databus23/
$ cd $GOPATH/src/github.com/databus23/
$ git clone https://github.com/databus23/helm-diff.git
$ cd helm-diff
$ make install
```

The above will install this plugin into your `$HELM_HOME/plugins` directory.

### Prerequisites

- You need to have [Go](http://golang.org) installed. Make sure to set `$GOPATH`

### Running Tests
Automated tests are implemented with [*testing*](https://golang.org/pkg/testing/).

To run all tests:
```
go test -v ./...
```

## Release

Bump `version` in `plugin.yaml`:

```
$ code plugin.yaml
$ git commit -m 'Bump helm-diff version to 3.x.y'
```

Set `GITHUB_TOKEN` and run:

```
$ make docker-run-release
```
