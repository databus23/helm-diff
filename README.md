# Helm Diff Plugin

This is a Helm plugin giving your a preview of what a `helm upgrade` would change.
It basically generates a diff between the latest deployed version of a release
and a `helm upgrade --debug --dry-run`

<a href="https://asciinema.org/a/105326" target="_blank"><img src="https://asciinema.org/a/105326.png" /></a>

## Usage

```
$ helm diff [flags] RELEASE CHART
```

### Flags:

```
      --set string          set values on the command line. See 'helm install -h'
  -f, --values valueFiles   specify one or more YAML files of values (default [])
```


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
