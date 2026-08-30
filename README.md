# Helm Diff Plugin
[![Go Report Card](https://goreportcard.com/badge/github.com/databus23/helm-diff)](https://goreportcard.com/report/github.com/databus23/helm-diff)
[![GoDoc](https://godoc.org/github.com/databus23/helm-diff?status.svg)](https://godoc.org/github.com/databus23/helm-diff)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/databus23/helm-diff/blob/master/LICENSE)
[![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/databus23/helm-diff)

This is a Helm plugin giving you a preview of what a `helm upgrade` would change.
It basically generates a diff between the latest deployed version of a release
and a `helm template`-rendered manifest (or `helm upgrade --dry-run` when `HELM_DIFF_USE_UPGRADE_DRY_RUN=true` is set).
This can also be used to compare two revisions/versions of your helm release.

<a href="https://asciinema.org/a/105326" target="_blank"><img src="https://asciinema.org/a/105326.png" /></a>

## Install

### Using Helm plugin manager (> 2.3.x)

*requires helm 3.18+*

```shell
helm plugin install https://github.com/databus23/helm-diff
```

### Installing offline

If installing this in an offline/airgapped environment, download the platform-specific binary archive (e.g., `helm-diff-linux-amd64.tgz` or `helm-diff-windows-amd64.tgz`) from [releases](https://github.com/databus23/helm-diff/releases). Make sure to select the correct `.tgz` file for your operating system and architecture.

The release archives include everything needed to install the plugin (binary, `plugin.yaml`, and the install scripts). The simplest way to install offline is to extract the archive and point `helm plugin install` at the extracted directory:

```
tar xzf helm-diff-linux-amd64.tgz   # extracts into a ./diff directory
helm plugin install ./diff
```

The install script detects that the binary is already bundled and skips the GitHub download.

Alternatively, if you keep a separate local checkout of the plugin source, you can point the installer at a downloaded `.tgz` via the `HELM_DIFF_BIN_TGZ` environment variable.

Set `HELM_DIFF_BIN_TGZ` to the absolute path to the downloaded binary archive:

**POSIX shell:**
```sh
export HELM_DIFF_BIN_TGZ=/path/to/helm-diff-linux-amd64.tgz


**PowerShell:**
```powershell
$env:HELM_DIFF_BIN_TGZ = "C:\path\to\helm-diff-bin.tgz"
```

Now, run `helm plugin install /path/to/helm-diff/`.
Here, `/path/to/helm-diff/` must be a local copy of the Helm Diff plugin source directory (including `plugin.yaml` and the install scripts), for example from a repo you cloned or a source archive you downloaded earlier and transferred into the offline environment.
The install script will skip the GitHub download and instead install from the `.tgz`.

**For Helm 4 users:**

Helm 4 verifies plugin provenance by default. This project publishes GPG-signed provenance artifacts (`.prov`) alongside release tarballs. To verify, import the project's public key into your keyring and install from a direct tarball URL (git repo URLs do not support provenance verification):

```shell
curl -sL https://github.com/databus23.gpg | gpg --import
gpg --list-keys --with-fingerprint EA17A2A206AFF8CD
# Expected fingerprint: C5645EF4 7482257A 1F806D2B EA17A2A2 06AFF8CD
helm plugin install https://github.com/databus23/helm-diff/releases/latest/download/helm-diff-linux-amd64.tgz
```

For offline/airgapped environments, download the public key from the maintainer's GitHub profile on a connected machine, transfer it, and import it locally:

```shell
curl -sL https://github.com/databus23.gpg -o pubkey.asc
gpg --import pubkey.asc
gpg --list-keys --with-fingerprint EA17A2A206AFF8CD
# Expected fingerprint: C5645EF4 7482257A 1F806D2B EA17A2A2 06AFF8CD
```

The public key fingerprint is published in the notes for each GitHub release.

For more information about Helm 4's plugin verification, see:
- [Helm 4 Overview](https://helm.sh/docs/overview)
- [HIP-0026: Plugin Provenance](https://github.com/helm/community/blob/main/hips/hip-0026.md)
- [Helm Provenance Documentation](https://helm.sh/docs/topics/provenance/)

### Pre Helm 2.3.0 Installation
Pick a release tarball from the [releases](https://github.com/databus23/helm-diff/releases) page.

Unpack the tarball in your helm plugins directory (`$(helm home)/plugins`).

E.g.
```
curl -L $TARBALL_URL | tar -C $(helm home)/plugins -xzv
```

### From Source
#### Prerequisites
 - GoLang `>= 1.21`

Make sure you do not have a version of `helm-diff` installed. You can remove it by running `helm plugin uninstall diff`

#### Installation Steps
The first step is to download the repository and enter the directory. You can do this via `git clone` or downloading and extracting the release. If you clone via git, remember to checkout the latest tag for the latest release.

Next, install the plugin into helm.

```bash
make install/helm
```


## Usage

```
The Helm Diff Plugin

* Shows a diff explaining what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be
  used to visualize what changes a helm upgrade will perform.

* Shows a diff explaining what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used to visualize what changes
  were made during revision change.

* Shows a diff explaining what a helm rollback would change:
    This fetches the currently deployed version of a release
  and compares it to the previously deployed version of the release, that you
  want to rollback. This can be used to visualize what changes a
  helm rollback will perform.

Usage:
  diff [flags]
  diff [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  local       Shows diff between two local chart directories
  release     Shows diff between release's manifests
  revision    Shows diff between revision's manifests
  rollback    Show a diff explaining what a helm rollback could perform
  upgrade     Show a diff explaining what a helm upgrade would change.
  version     Show version of the helm diff plugin

Flags:
      --allow-unreleased                         enables diffing of releases that are not yet deployed via Helm
  -a, --api-versions stringArray                 Kubernetes api versions used for Capabilities.APIVersions
      --color                                    color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --devel                                    use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
      --disable-openapi-validation               disables rendered templates validation against the Kubernetes OpenAPI Schema
      --disable-validation                       disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install
      --dry-run string[="client"]                --dry-run, --dry-run=client, or --dry-run=true disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation. --dry-run=server enables the cluster access with helm-get and the lookup template function.
      --enable-dns                               enable DNS lookups when rendering templates
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for diff
      --include-crds                             include CRDs in the diffing
      --include-tests                            enable the diffing of the helm test hooks
      --insecure-skip-tls-verify                 skip tls certificate checks for the chart download
      --install                                  enables diffing of releases that are not yet deployed via Helm (equivalent to --allow-unreleased, added to match "helm upgrade --install" command
      --kube-context string                      name of the kubeconfig context to use
      --kube-version string                      Kubernetes version used for Capabilities.KubeVersion
      --kubeconfig string                        This flag is ignored, to allow passing of this top level flag to helm
  -n, --namespace string                         namespace to assume the release to be installed into. Defaults to the current kube config namespace.
      --no-color                                 remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-hooks                                 disable diffing of hooks
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --post-renderer string                     the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path
      --post-renderer-args stringArray           an argument to the post-renderer (can specify multiple)
      --repo string                              specify the chart repository url to locate the requested chart
      --reset-then-reuse-values                  reset the values to the ones built into the chart, apply the last release's values and merge in any new values. If '--reset-values' or '--reuse-values' is specified, this is ignored
      --reset-values                             reset the values to the ones built into the chart and merge in any new values
      --reuse-values                             reuse the last release's values and merge in any new values. If '--reset-values' is specified, this is ignored
      --revision int                             revision of the release to use as the diff baseline instead of the newest one
      --server-side string                       must be "true", "false" or "auto". Object updates run in the server instead of the client ("auto" defaults the value from the previous chart release's method) (default "auto")
      --set stringArray                          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray                     set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-json stringArray                     set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
      --set-literal stringArray                  set STRING literal values on the command line
      --set-string stringArray                   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --skip-schema-validation                   skip validation of the rendered manifests against the Kubernetes OpenAPI schema
      --storage-namespace string                 namespace where the helm release storage (Secret/ConfigMap) is located. Defaults to the target namespace (-n/--namespace)
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output
      --take-ownership                           if set, upgrade will ignore the check for helm annotations and take ownership of the existing resources
      --three-way-merge                          use three-way-merge to compute patch and generate diff output
  -f, --values valueFiles                        specify values in a YAML file (can specify multiple) (default [])
      --version string                           specify the exact chart version to use. If this is not specified, the latest version is used

Additional help topics:
  diff

Use "diff [command] --help" for more information about a command.
```

### Structured JSON output

Set `--output structured` (or `HELM_DIFF_OUTPUT=structured`) to emit machine-readable JSON. Each entry reports the Kubernetes object metadata, resource existence, and per-field changes using JSON Pointer paths:

```shell
helm diff upgrade prod api ./charts/api --output structured
```

```json
[
  {
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "namespace": "prod",
    "name": "api",
    "changeType": "MODIFY",
    "resourceStatus": {"oldExists": true, "newExists": true},
    "changes": [
      {"path": "spec", "field": "replicas", "change": "replace", "oldValue": 2, "newValue": 3},
      {"path": "spec.template.spec.containers[0]", "field": "image", "change": "replace", "oldValue": "api:v1", "newValue": "api:v2"}
    ]
  }
]
```

When a kind is suppressed via `--suppress`, `changesSuppressed` is set to `true` and field details are omitted. Nested metadata such as labels show the container path (`metadata.labels`) and expose the label key through the `field` property (for example `app.kubernetes.io/version`).

### External diff tool

Set `--diff-tool` to a command and helm-diff renders the diff with that command instead of its built-in renderers. It writes the old and the new manifests into two temporary files (named `old.yaml` and `new.yaml` in a private temporary directory) and appends their paths as the last two arguments:

```shell
# any tool that accepts two file paths works
helm diff upgrade api ./charts/api --diff-tool "diff -u -N"
helm diff upgrade api ./charts/api --diff-tool "difft --language yaml"
helm diff upgrade api ./charts/api --diff-tool "git --no-pager diff --no-index --color"
helm diff upgrade api ./charts/api --diff-tool "delta --side-by-side"
```

The command can also be set through the `HELM_DIFF_TOOL` environment variable, which is convenient in a shell profile:

```shell
export HELM_DIFF_TOOL="difft --language yaml"
helm diff upgrade api ./charts/api
```

`--diff-tool` overrides both `--output` and `HELM_DIFF_TOOL`. `HELM_DIFF_TOOL` applies only when neither `--output` nor `--diff-tool` is explicitly given, so scripts that parse a specific output format cannot be broken by a variable inherited from a shell profile; an explicitly empty `--diff-tool ""` disables the external tool. There is no default command: without one, the built-in `--output` renderer is used.

Notes:

- The command is executed directly, not through a shell, so pipes and shell expansion are not available. Wrap arguments containing spaces in quotes, for example `--diff-tool '"/opt/my tools/diff" -u'`; an unclosed quote is rejected with an error. For anything more involved, point the flag at a wrapper script.
- Each resource in the temporary files is preceded by a `# Resource:`/`# Change:` header comment (change types: `ADD`, `REMOVE`, `MODIFY`, `OWNERSHIP`, and `MODIFY_SUPPRESSED` when the diff is empty after `--suppress-output-line-regex`), because an external tool would otherwise have no way to show them.
- The manifests handed to the tool are the ones from the diff report, so `--suppress`, `--suppress-output-line-regex` and secret redaction still apply. Secrets are redacted unless `--show-secrets` is given; suppressed kinds — and entries whose diff is empty after `--suppress-output-line-regex` — are replaced by a placeholder on both sides.
- The command must block until it has finished reading the two files. GUI tools that return immediately (for example `code --diff`) may find the temporary directory already deleted before they display it; make them wait (for example `code --wait --diff`) or wrap them in a script that waits.
- There is no timeout around the command: a tool that never exits keeps helm-diff running.
- An exit code of `1` from the tool is treated as "differences found" and ignored. Other failures are reported on stderr without aborting helm-diff.
- helm-diff's own exit code is unaffected by the tool: `--detailed-exitcode` still returns `2` based on the changes helm-diff detected.
- `--context`/`-C` is not applied; use the equivalent option of the external tool (for example `diff -U3`).

## Commands:

### local:

```
$ helm diff local -h

This command compares the manifests of two local chart directories.

It renders both charts using 'helm template' and shows the differences
between the resulting manifests.

This is useful for:
 - Comparing different versions of a chart
 - Previewing changes before committing
 - Validating chart modifications

Usage:
  diff local [flags] CHART1 CHART2

Examples:
  helm diff local ./chart-v1 ./chart-v2
  helm diff local ./chart-v1 ./chart-v2 -f values.yaml
  helm diff local /path/to/chart-a /path/to/chart-b --set replicas=3

Flags:
  -a, --api-versions stringArray                 Kubernetes api versions used for Capabilities.APIVersions
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
      --enable-dns                               enable DNS lookups when rendering templates
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for local
      --include-crds                             include CRDs in the diffing
      --include-tests                            enable the diffing of the helm test hooks
      --kube-version string                      Kubernetes version used for Capabilities.KubeVersion
      --namespace string                         namespace to use for template rendering
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --post-renderer string                     the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path
      --post-renderer-args stringArray           an argument to the post-renderer (can specify multiple)
      --release string                           release name to use for template rendering (default "release")
      --set stringArray                          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray                     set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-json stringArray                     set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
      --set-literal stringArray                  set STRING literal values on the command line
      --set-string stringArray                   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output
  -f, --values valueFiles                        specify values in a YAML file (can specify multiple) (default [])

Global Flags:
      --color      color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-color   remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
```

### upgrade:

```
$ helm diff upgrade -h
Show a diff explaining what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a chart plus values.
This can be used to visualize what changes a helm upgrade will
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

  # Set HELM_DIFF_NORMALIZE_MANIFESTS=true to
  # normalize the yaml file content when using helm diff.
  # This is equivalent to specifying the --normalize-manifests flag.
  # Read the flag usage below for more information on --normalize-manifests.
  HELM_DIFF_NORMALIZE_MANIFESTS=true helm diff upgrade my-release datadog/datadog

# Set HELM_DIFF_OUTPUT_CONTEXT=n to configure the output context to n lines.
# This is equivalent to specifying the --context flag.
# Read the flag usage below for more information on --context.
HELM_DIFF_OUTPUT_CONTEXT=5 helm diff upgrade my-release datadog/datadog

  # Set HELM_DIFF_STORAGE_NAMESPACE=flux-system to
  # fetch release manifests/values/hooks from a storage namespace different from the target namespace.
  # This is equivalent to specifying the --storage-namespace flag.
  HELM_DIFF_STORAGE_NAMESPACE=flux-system helm diff upgrade -n prod-apps my-release datadog/datadog

  # NOTE: The storage namespace separation is not supported in combination with
  # HELM_DIFF_USE_UPGRADE_DRY_RUN=true, because rendering then goes through
  # `helm upgrade --dry-run`, which resolves the release storage in the target
  # namespace. If the release exists only in the storage namespace, keep the
  # default `helm template` based rendering instead.

Flags:
      --allow-unreleased                         enables diffing of releases that are not yet deployed via Helm
  -a, --api-versions stringArray                 Kubernetes api versions used for Capabilities.APIVersions
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --devel                                    use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
      --disable-openapi-validation               disables rendered templates validation against the Kubernetes OpenAPI Schema
      --disable-validation                       disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install
      --dry-run string[="client"]                --dry-run, --dry-run=client, or --dry-run=true disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation. --dry-run=server enables the cluster access with helm-get and the lookup template function.
      --enable-dns                               enable DNS lookups when rendering templates
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for upgrade
      --include-crds                             include CRDs in the diffing
      --include-tests                            enable the diffing of the helm test hooks
      --insecure-skip-tls-verify                 skip tls certificate checks for the chart download
      --install                                  enables diffing of releases that are not yet deployed via Helm (equivalent to --allow-unreleased, added to match "helm upgrade --install" command
      --kube-context string                      name of the kubeconfig context to use
      --kube-version string                      Kubernetes version used for Capabilities.KubeVersion
      --kubeconfig string                        This flag is ignored, to allow passing of this top level flag to helm
  -n, --namespace string                         namespace to assume the release to be installed into. Defaults to the current kube config namespace.
      --no-hooks                                 disable diffing of hooks
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --post-renderer string                     the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path
      --post-renderer-args stringArray           an argument to the post-renderer (can specify multiple)
      --repo string                              specify the chart repository url to locate the requested chart
      --reset-then-reuse-values                  reset the values to the ones built into the chart, apply the last release's values and merge in any new values. If '--reset-values' or '--reuse-values' is specified, this is ignored
      --reset-values                             reset the values to the ones built into the chart and merge in any new values
      --reuse-values                             reuse the last release's values and merge in any new values. If '--reset-values' is specified, this is ignored
      --revision int                             revision of the release to use as the diff baseline instead of the newest one
      --server-side string                       must be "true", "false" or "auto". Object updates run in the server instead of the client ("auto" defaults the value from the previous chart release's method) (default "auto")
      --set stringArray                          set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray                     set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-json stringArray                     set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
      --set-literal stringArray                  set STRING literal values on the command line
      --set-string stringArray                   set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --skip-schema-validation                   skip validation of the rendered manifests against the Kubernetes OpenAPI schema
      --storage-namespace string                 namespace where the helm release storage (Secret/ConfigMap) is located. Defaults to the target namespace (-n/--namespace)
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output
      --take-ownership                           if set, upgrade will ignore the check for helm annotations and take ownership of the existing resources
      --three-way-merge                          use three-way-merge to compute patch and generate diff output
  -f, --values valueFiles                        specify values in a YAML file (can specify multiple) (default [])
      --version string                           specify the exact chart version to use. If this is not specified, the latest version is used

Global Flags:
      --color      color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-color   remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
```

### release:

```
$ helm diff release -h

This command compares the manifests details of a different releases created from the same chart.
The release name may be specified using namespace/release syntax.

It can be used to compare the manifests of

 - release1 with release2
        $ helm diff release [flags] release1 release2
   Example:
        $ helm diff release my-prod my-stage
        $ helm diff release prod/my-prod stage/my-stage

Usage:
  diff release [flags] RELEASE release1 [release2]

Flags:
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for release
      --include-tests                            enable the diffing of the helm test hooks
      --kube-context string                      name of the kubeconfig context to use
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output

Global Flags:
      --color      color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-color   remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
```

### revision:

```
$ helm diff revision -h

This command compares the manifests details of a named release.

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
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for revision
      --include-tests                            enable the diffing of the helm test hooks
      --kube-context string                      name of the kubeconfig context to use
  -n, --namespace string                         namespace to assume the release to be installed into. Defaults to the current kube config namespace.
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --storage-namespace string                 namespace where the helm release storage (Secret/ConfigMap) is located. Defaults to the target namespace (-n/--namespace)
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output

Global Flags:
      --color      color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-color   remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
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
  -C, --context int                              output NUM lines of context around changes (default -1)
      --detailed-exitcode                        return a non-zero exit code when there are changes
      --diff-tool string                         command used to compare the manifests instead of the built-in --output renderers (can also be set via the env var HELM_DIFF_TOOL). The old and the new manifest file paths are appended as the last two arguments
  -D, --find-renames float32                     Enable rename detection if set to any value greater than 0. If specified, the value denotes the maximum fraction of changed content as lines added + removed compared to total lines in a diff for considering it a rename. Only objects of the same Kind are attempted to be matched
  -h, --help                                     help for rollback
      --include-tests                            enable the diffing of the helm test hooks
      --kube-context string                      name of the kubeconfig context to use
  -n, --namespace string                         namespace to assume the release to be installed into. Defaults to the current kube config namespace.
      --normalize-manifests                      normalize manifests before running diff to exclude style differences from the output
      --output string                            Possible values: diff, simple, template, json, structured, dyff. When set to "template", use the env var HELM_DIFF_TPL to specify the template. (default "diff")
      --show-secrets                             do not redact secret values in the output
      --show-secrets-decoded                     decode secret values in the output
      --storage-namespace string                 namespace where the helm release storage (Secret/ConfigMap) is located. Defaults to the target namespace (-n/--namespace)
      --strip-trailing-cr                        strip trailing carriage return on input
      --suppress stringArray                     allows suppression of the kinds listed in the diff output (can specify multiple, like '--suppress Deployment --suppress Service')
      --suppress-output-line-regex stringArray   a regex to suppress diff output lines that match
  -q, --suppress-secrets                         suppress secrets in the output

Global Flags:
      --color      color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
      --no-color   remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not "dumb"
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

### Updating the flag tables in this README

The per-command `Flags:` tables above are generated from the actual `--help`
output. After adding or changing a command flag, regenerate them with:
```
make readme
```
CI fails if the committed tables do not match the binary (`make verify-readme`).

## Release

Bump `version` in `plugin.yaml`:

```
$ code plugin.yaml
$ git commit -s -m 'Bump helm-diff version to 3.x.y'
```

Set `GITHUB_TOKEN` and run:

```
$ make docker-run-release
```
