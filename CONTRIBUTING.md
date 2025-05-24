Before submitting a pull request, I'd encourage you to test it yourself.

To do so, you need to run the plugin indirectly or directly.

**Indirect** Install the plugin locally and run it via helm:

```
$ helm plugin uninstall diff

$ helm plugin list
#=> Make sure that the previous installation of helm-diff has unisntalled

$ make install

$ helm plugin list
#=> Make sure that the version of helm-diff built from your branch has instaled

$ helm diff upgrade ... (snip)
```

**Direct** Build the plugin binary and execute it with a few helm-specific environment variables:

```
$ go build .

$ HELM_NAMESPACE=default \
HELM_BIN=helm372 \
  ./helm-diff upgrade foo $CHART \
  --set argo-cd.nameOverride=testtest \
  --install
```
