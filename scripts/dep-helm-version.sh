#/bin/sh

grep "k8s.io/helm" go.mod | sed -n -e "s/.*k8s.io\/helm \(v[.0-9]*\).*/\1/p"
