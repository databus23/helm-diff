# Helm Diff Plugin

This is a Helm plugin giving your a preview of what a `helm upgrade` would change.
It basically generates a diff between the latest deployed version of a release
and a `helm upgrade --debug --dry-run`. This can also be used to compare two 
revisions/versions of your helm release.

<a href="https://asciinema.org/a/105326" target="_blank"><img src="https://asciinema.org/a/105326.png" /></a>

## Usage

```
The Helm Diff Plugin

* Shows a diff explaing what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be 
  used visualize what changes a helm upgrade will perform.

* Shows a diff explaing what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used visualize what changes 
  were made during revision change.

* Shows a diff explaing what a helm rollback would change:
    This fetches the currently deployed version of a release
  and compares it to adeployed versions of a release, that you 
  want to rollback. This can be used visualize what changes a 
  helm rollback will perform.

Usage:
  diff [flags]
  diff [command]

Available Commands:
  revision    Shows diff between revision's manifests
  rollback    Show a diff explaining what a helm rollback could perform
  upgrade     Show a diff explaining what a helm upgrade would change.
  version     Show version of the helm diff plugin

Flags:
  -h, --help                   help for diff
      --no-color               remove colors from the output
      --reset-values           reset the values to the ones built into the chart and merge in any new values
      --reuse-values           reuse the last release's values and merge in any new values
      --set stringArray        set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output
  -f, --values valueFiles      specify values in a YAML file (can specify multiple) (default [])
      --version string         specify the exact chart version to use. If this is not specified, the latest version is used

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

Flags:
  -h, --help                   help for upgrade
      --detailed-exitcode      return a non-zero exit code when there are changes
      --reset-values           reset the values to the ones built into the chart and merge in any new values
      --reuse-values           reuse the last release's values and merge in any new values
      --set stringArray        set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output
  -f, --values valueFiles      specify values in a YAML file (can specify multiple) (default [])
      --version string         specify the exact chart version to use. If this is not specified, the latest version is used

Global Flags:
      --no-color   remove colors from the output
```

### revision:

```
$ helm diff revision -h

This command compares the manifests details of a named release.

It can be used to compare the manifests of 
 
 - lastest REVISION with specified REVISION
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
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output

Global Flags:
      --no-color   remove colors from the output
```

### rollback:

```
$ helm diff rollback -h

This command compares the laset manifests details of a named release 
with specific revision values to rollback.

It forecasts/visualizes changes, that a helm rollback could perform.

Usage:
  diff rollback [flags] [RELEASE] [REVISION]

Examples:
  helm diff rollback my-release 2

Flags:
  -h, --help                   help for rollback
      --suppress stringArray   allows suppression of the values listed in the diff output
  -q, --suppress-secrets       suppress secrets in the output

Global Flags:
      --no-color   remove colors from the output
```


## Install

### Using Helm plugin manager (> 2.3.x)

```shell
helm plugin install https://github.com/databus23/helm-diff --version master
```

### Pre Helm 2.3.0 Installation
Pick a release tarball from the [releases](https://github.com/databus23/helm-diff/releases) page.

Unpack the tarball in your helm plugins directory (`$(helm home)/plugins`).

E.g.
```
curl -L $TARBALL_URL | tar -C $(helm home)/plugins -xzv
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
- If you don't have [Glide](http://glide.sh) installed, this will install it into
  `$GOPATH/bin` for you.

### Running Tests
Automated tests are implemented with [*testing*](https://golang.org/pkg/testing/).

To run all tests:
```
go test -v ./...
```
