export GITHUB_TOKEN=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
export pkg=/go/src/github.com/ksa-real/helm-diff
docker run -it --rm -e GITHUB_TOKEN -v $(pwd):$pkg -w $pkg golang:1.17.5 bash
