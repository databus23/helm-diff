package diff

import (
	"bytes"
	"os"
	"testing"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"
	"github.com/stretchr/testify/require"

	"github.com/databus23/helm-diff/v3/manifest"
)

var text1 = "" +
	"line1\n" +
	"line2\n" +
	"line3\n" +
	"line4\n" +
	"line5\n" +
	"line6\n" +
	"line7\n" +
	"line8\n" +
	"line9\n" +
	"line10"

var text2 = "" +
	"line1 - different!\n" +
	"line2 - different!\n" +
	"line3\n" +
	"line4\n" +
	"line5\n" +
	"line6\n" +
	"line7\n" +
	"line8 - different!\n" +
	"line9\n" +
	"line10"

var text3 = "" +
	"line1\r\n" +
	"line2\r\n" +
	"line3\r\n" +
	"line4\r\n" +
	"line5\r\n" +
	"line6\r\n" +
	"line7\r\n" +
	"line8\r\n" +
	"line9\r\n" +
	"line10"

func TestPrintDiffWithContext(t *testing.T) {
	t.Run("context-disabled", func(t *testing.T) {
		assertDiff(t, text1, text2, -1, false, ""+
			"- line1\n"+
			"- line2\n"+
			"+ line1 - different!\n"+
			"+ line2 - different!\n"+
			"  line3\n"+
			"  line4\n"+
			"  line5\n"+
			"  line6\n"+
			"  line7\n"+
			"- line8\n"+
			"+ line8 - different!\n"+
			"  line9\n"+
			"  line10\n")
	})

	t.Run("context-0", func(t *testing.T) {
		assertDiff(t, text1, text2, 0, false, ""+
			"- line1\n"+
			"- line2\n"+
			"+ line1 - different!\n"+
			"+ line2 - different!\n"+
			"...\n"+
			"- line8\n"+
			"+ line8 - different!\n"+
			"...\n")
	})

	t.Run("context-0-no-strip-cr", func(t *testing.T) {
		assertDiff(t, text1, text3, 0, false, ""+
			"- line1\n"+
			"- line2\n"+
			"- line3\n"+
			"- line4\n"+
			"- line5\n"+
			"- line6\n"+
			"- line7\n"+
			"- line8\n"+
			"- line9\n"+
			"+ line1\r\n"+
			"+ line2\r\n"+
			"+ line3\r\n"+
			"+ line4\r\n"+
			"+ line5\r\n"+
			"+ line6\r\n"+
			"+ line7\r\n"+
			"+ line8\r\n"+
			"+ line9\r\n"+
			"...\n")
	})

	t.Run("context-0-strip-cr", func(t *testing.T) {
		assertDiff(t, text1, text3, 0, true, ""+
			"...\n")
	})

	t.Run("context-1", func(t *testing.T) {
		assertDiff(t, text1, text2, 1, false, ""+
			"- line1\n"+
			"- line2\n"+
			"+ line1 - different!\n"+
			"+ line2 - different!\n"+
			"  line3\n"+
			"...\n"+
			"  line7\n"+
			"- line8\n"+
			"+ line8 - different!\n"+
			"  line9\n"+
			"...\n")
	})

	t.Run("context-2", func(t *testing.T) {
		assertDiff(t, text1, text2, 2, false, ""+
			"- line1\n"+
			"- line2\n"+
			"+ line1 - different!\n"+
			"+ line2 - different!\n"+
			"  line3\n"+
			"  line4\n"+
			"...\n"+
			"  line6\n"+
			"  line7\n"+
			"- line8\n"+
			"+ line8 - different!\n"+
			"  line9\n"+
			"  line10\n")
	})

	t.Run("context-3", func(t *testing.T) {
		assertDiff(t, text1, text2, 3, false, ""+
			"- line1\n"+
			"- line2\n"+
			"+ line1 - different!\n"+
			"+ line2 - different!\n"+
			"  line3\n"+
			"  line4\n"+
			"  line5\n"+
			"  line6\n"+
			"  line7\n"+
			"- line8\n"+
			"+ line8 - different!\n"+
			"  line9\n"+
			"  line10\n")
	})
}

func assertDiff(t *testing.T, before, after string, context int, stripTrailingCR bool, expected string) {
	ansi.DisableColors(true)
	var output bytes.Buffer
	diffs := diffStrings(before, after, stripTrailingCR)
	printDiffRecords([]string{}, "some-resource", context, diffs, &output)
	actual := output.String()
	if actual != expected {
		t.Errorf("Unexpected diff output: \nExpected:\n#%v# \nActual:\n#%v#", expected, actual)
	}
}

func TestManifests(t *testing.T) {
	ansi.DisableColors(true)

	specBeta := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {

			Name: "default, nginx, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nginx
`,
		}}

	specRelease := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {

			Name: "default, nginx, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
`,
		}}

	specReleaseSpec := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {

			Name: "default, nginx, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 3
`,
		}}

	specReleaseRenamed := map[string]*manifest.MappingResult{
		"default, nginx-renamed, Deployment (apps)": {

			Name: "default, nginx-renamed, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-renamed
spec:
  replicas: 3
`,
		}}

	specReleaseRenamedAndUpdated := map[string]*manifest.MappingResult{
		"default, nginx-renamed, Deployment (apps)": {

			Name: "default, nginx-renamed, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-renamed
spec:
  replicas: 1
`,
		}}

	specReleaseRenamedAndAdded := map[string]*manifest.MappingResult{
		"default, nginx-renamed, Deployment (apps)": {

			Name: "default, nginx-renamed, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-renamed
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx-renamed
`,
		}}

	specReleaseKeep := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {

			Name: "default, nginx, Deployment (apps)",
			Kind: "Deployment",
			Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
annotations:
  helm.sh/resource-policy: keep
`,
			ResourcePolicy: "keep",
		}}

	t.Run("OnChange", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

- apiVersion: apps/v1beta1
+ apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: nginx

`, buf1.String())
	})

	t.Run("OnChangeWithSuppress", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.0, []string{"apiVersion"}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:
`, buf1.String())
	})

	t.Run("OnChangeRename", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{}}

		if changesSeen := Manifests(specReleaseSpec, specReleaseRenamed, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx
+   name: nginx-renamed
  spec:
    replicas: 3

`, buf1.String())
	})

	t.Run("OnChangeRenameAndUpdate", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{}}

		if changesSeen := Manifests(specReleaseSpec, specReleaseRenamedAndUpdated, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx
+   name: nginx-renamed
  spec:
-   replicas: 3
+   replicas: 1

`, buf1.String())
	})

	t.Run("OnChangeRenameAndAdded", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{}}

		if changesSeen := Manifests(specReleaseSpec, specReleaseRenamedAndAdded, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx
+   name: nginx-renamed
  spec:
    replicas: 3
+   selector:
+     matchLabels:
+       app: nginx-renamed

`, buf1.String())
	})

	t.Run("OnChangeRenameAndAddedWithPartialSuppress", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{"app: "}}

		if changesSeen := Manifests(specReleaseSpec, specReleaseRenamedAndAdded, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx
+   name: nginx-renamed
  spec:
    replicas: 3
+   selector:
+     matchLabels:

`, buf1.String())
	})

	t.Run("OnChangeRenameAndRemoved", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{}}

		if changesSeen := Manifests(specReleaseRenamedAndAdded, specReleaseSpec, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx-renamed, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx-renamed
+   name: nginx
  spec:
    replicas: 3
-   selector:
-     matchLabels:
-       app: nginx-renamed

`, buf1.String())
	})

	t.Run("OnChangeRenameAndRemovedWithPartialSuppress", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{"app: "}}

		if changesSeen := Manifests(specReleaseRenamedAndAdded, specReleaseSpec, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx-renamed, Deployment (apps) has changed:

  apiVersion: apps/v1
  kind: Deployment
  metadata:
-   name: nginx-renamed
+   name: nginx
  spec:
    replicas: 3
-   selector:
-     matchLabels:

`, buf1.String())
	})

	t.Run("OnNoChange", func(t *testing.T) {
		var buf2 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, ``, buf2.String())
	})

	t.Run("OnChangeRemoved", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.5, []string{}}

		if changesSeen := Manifests(specRelease, nil, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has been removed:

- apiVersion: apps/v1
- kind: Deployment
- metadata:
-   name: nginx
- `+`
`, buf1.String())
	})

	t.Run("OnChangeRemovedWithResourcePolicyKeep", func(t *testing.T) {
		var buf2 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specReleaseKeep, nil, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, ``, buf2.String())
	})

	t.Run("OnChangeSimple", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"simple", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) to be changed.
Plan: 0 to add, 1 to change, 0 to destroy.
`, buf1.String())
	})

	t.Run("OnNoChangeSimple", func(t *testing.T) {
		var buf2 bytes.Buffer
		diffOptions := Options{"simple", 10, false, true, []string{}, 0.0, []string{}}
		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Plan: 0 to add, 0 to change, 0 to destroy.\n", buf2.String())
	})

	t.Run("OnChangeTemplate", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"template", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `[{
  "api": "apps",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "change": "MODIFY"
}]
`, buf1.String())
	})

	t.Run("OnChangeJSON", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"json", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `[{
  "api": "apps",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "change": "MODIFY"
}]
`, buf1.String())
	})

	t.Run("OnNoChangeTemplate", func(t *testing.T) {
		var buf2 bytes.Buffer
		diffOptions := Options{"template", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "[]\n", buf2.String())
	})

	t.Run("OnChangeCustomTemplate", func(t *testing.T) {
		var buf1 bytes.Buffer
		os.Setenv("HELM_DIFF_TPL", "testdata/customTemplate.tpl")
		diffOptions := Options{"template", 10, false, true, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Resource name: nginx\n", buf1.String())
	})
}

func TestManifestsWithRedactedSecrets(t *testing.T) {
	ansi.DisableColors(true)

	specSecretWithByteData := map[string]*manifest.MappingResult{
		"default, foobar, Secret (v1)": {
			Name: "default, foobar, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foobar
type: Opaque
data:
  key1: dmFsdWUx
  key2: dmFsdWUy
  key3: dmFsdWUz
`,
		}}

	specSecretWithByteDataChanged := map[string]*manifest.MappingResult{
		"default, foobar, Secret (v1)": {
			Name: "default, foobar, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foobar
type: Opaque
data:
  key1: dmFsdWUxY2hhbmdlZA==
  key2: dmFsdWUy
  key4: dmFsdWU0
`,
		}}

	specSecretWithStringData := map[string]*manifest.MappingResult{
		"default, foobar, Secret (v1)": {
			Name: "default, foobar, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foobar
type: Opaque
stringData:
  key1: value1
  key2: value2
  key3: value3
`,
		}}

	specSecretWithStringDataChanged := map[string]*manifest.MappingResult{
		"default, foobar, Secret (v1)": {
			Name: "default, foobar, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foobar
type: Opaque
stringData:
  key1: value1changed
  key2: value2
  key4: value4
`,
		}}

	t.Run("OnChangeSecretWithByteData", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, false, []string{}, 0.5, []string{}} //NOTE: ShowSecrets = false

		if changesSeen := Manifests(specSecretWithByteData, specSecretWithByteDataChanged, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		//TODO: Why is there no empty line between the header and the start of the diff, like in the other diffs?
		require.Equal(t, `default, foobar, Secret (v1) has changed:
  apiVersion: v1
  kind: Secret
  metadata:
    name: foobar
  data:
-   key1: '-------- # (6 bytes)'
+   key1: '++++++++ # (13 bytes)'
    key2: 'REDACTED # (6 bytes)'
-   key3: '-------- # (6 bytes)'
+   key4: '++++++++ # (6 bytes)'
  type: Opaque

`, buf1.String())
	})

	t.Run("OnChangeSecretWithStringData", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, false, []string{}, 0.5, []string{}} //NOTE: ShowSecrets = false

		if changesSeen := Manifests(specSecretWithStringData, specSecretWithStringDataChanged, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, foobar, Secret (v1) has changed:
  apiVersion: v1
  kind: Secret
  metadata:
    name: foobar
  data:
-   key1: '-------- # (6 bytes)'
+   key1: '++++++++ # (13 bytes)'
    key2: 'REDACTED # (6 bytes)'
-   key3: '-------- # (6 bytes)'
+   key4: '++++++++ # (6 bytes)'
  type: Opaque

`, buf1.String())
	})
}

func TestDoSuppress(t *testing.T) {
	for _, tt := range []struct {
		name         string
		input        Report
		supressRegex []string
		expected     Report
	}{
		{
			name:         "noop",
			input:        Report{},
			supressRegex: []string{},
			expected:     Report{},
		},
		{
			name: "simple",
			input: Report{
				entries: []ReportEntry{
					{
						diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{},
			expected: Report{
				entries: []ReportEntry{
					{
						diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
		},
		{
			name: "ignore all",
			input: Report{
				entries: []ReportEntry{
					{
						diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{".*world2?"},
			expected: Report{
				entries: []ReportEntry{
					{
						diffs: []difflib.DiffRecord{},
					},
				},
			},
		},
		{
			name: "ignore partial",
			input: Report{
				entries: []ReportEntry{
					{
						diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{".*world2"},
			expected: Report{
				entries: []ReportEntry{
					{
						diffs: []difflib.DiffRecord{
							{
								Payload: "hello: world",
								Delta:   difflib.LeftOnly,
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := doSuppress(tt.input, tt.supressRegex)
			require.NoError(t, err)

			require.Equal(t, tt.expected, report)
		})
	}
}
