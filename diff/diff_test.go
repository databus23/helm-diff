package diff

import (
	"bytes"
	"encoding/json"
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
		},
	}

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
		},
	}

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
		},
	}

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
		},
	}

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
		},
	}

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
		},
	}

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
		},
	}

	t.Run("OnChange", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.0, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.0, []string{"apiVersion"}}

		if changesSeen := Manifests(specBeta, specReleaseSpec, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed:

  kind: Deployment
  metadata:
    name: nginx
+ spec:
+   replicas: 3

`, buf1.String())
	})

	t.Run("OnChangeWithSuppressAll", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.0, []string{"apiVersion"}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) has changed, but diff is empty after suppression.
`, buf1.String())
	})

	t.Run("OnChangeRename", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{"app: "}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{"app: "}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Empty(t, buf2.String())
	})

	t.Run("OnChangeRemoved", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}}

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
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specReleaseKeep, nil, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Empty(t, buf2.String())
	})

	t.Run("OnChangeSimple", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"simple", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) to be changed.
Plan: 0 to add, 1 to change, 0 to destroy, 0 to change ownership.
`, buf1.String())
	})

	t.Run("OnNoChangeSimple", func(t *testing.T) {
		var buf2 bytes.Buffer
		diffOptions := Options{"simple", 10, false, true, false, []string{}, 0.0, []string{}}
		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Plan: 0 to add, 0 to change, 0 to destroy, 0 to change ownership.\n", buf2.String())
	})

	t.Run("OnChangeTemplate", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"template", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.JSONEq(t, `[{
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
		diffOptions := Options{"json", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.JSONEq(t, `[{
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
		diffOptions := Options{"template", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specRelease, specRelease, &diffOptions, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "[]\n", buf2.String())
	})

	t.Run("OnChangeCustomTemplate", func(t *testing.T) {
		var buf1 bytes.Buffer
		os.Setenv("HELM_DIFF_TPL", "testdata/customTemplate.tpl")
		diffOptions := Options{"template", 10, false, true, false, []string{}, 0.0, []string{}}

		if changesSeen := Manifests(specBeta, specRelease, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Resource name: nginx\n", buf1.String())
	})
}

func TestStructuredOutputModify(t *testing.T) {
	ansi.DisableColors(true)
	opts := &Options{OutputFormat: "structured"}
	oldManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: app
        image: demo:v1
`
	newManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: demo:v2
`
	oldIndex := manifest.Parse(oldManifest, "prod", true)
	newIndex := manifest.Parse(newManifest, "prod", true)

	var buf bytes.Buffer
	changed := Manifests(oldIndex, newIndex, opts, &buf)
	require.True(t, changed)

	var entries []StructuredEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	entry := entries[0]
	require.Equal(t, "MODIFY", entry.ChangeType)
	require.Equal(t, "apps/v1", entry.APIVersion)
	require.Equal(t, "Deployment", entry.Kind)
	require.Equal(t, "prod", entry.Namespace)
	require.Equal(t, "web", entry.Name)
	require.Len(t, entry.Changes, 2)
	replicasChange, ok := findChange(entry.Changes, "spec", "replicas")
	require.True(t, ok)
	require.InDelta(t, float64(2), replicasChange.OldValue, 0.001)
	require.InDelta(t, float64(3), replicasChange.NewValue, 0.001)

	imageChange, ok := findChange(entry.Changes, "spec.template.spec.containers[0]", "image")
	require.True(t, ok)
	require.Equal(t, "demo:v1", imageChange.OldValue)
	require.Equal(t, "demo:v2", imageChange.NewValue)
}

func TestStructuredOutputAddAndRemove(t *testing.T) {
	ansi.DisableColors(true)
	opts := &Options{OutputFormat: "structured"}
	newManifest := `
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate
  namespace: ops
spec: {}
`
	newIndex := manifest.Parse(newManifest, "ops", true)

	var buf bytes.Buffer
	changed := Manifests(map[string]*manifest.MappingResult{}, newIndex, opts, &buf)
	require.True(t, changed)

	var entries []StructuredEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "ADD", entries[0].ChangeType)
	require.True(t, entries[0].ResourceStatus.NewExists)
	require.False(t, entries[0].ResourceStatus.OldExists)

	// Now test removal
	buf.Reset()
	changed = Manifests(newIndex, map[string]*manifest.MappingResult{}, opts, &buf)
	require.True(t, changed)
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "REMOVE", entries[0].ChangeType)
	require.True(t, entries[0].ResourceStatus.OldExists)
	require.False(t, entries[0].ResourceStatus.NewExists)
}

func TestStructuredOutputSuppressedKind(t *testing.T) {
	ansi.DisableColors(true)
	opts := &Options{
		OutputFormat:    "structured",
		SuppressedKinds: []string{"Secret"},
	}
	oldManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: creds
data:
  password: c29tZQ==
`
	newManifest := `
apiVersion: v1
kind: Secret
metadata:
  name: creds
data:
  password: Zm9v
`
	oldIndex := manifest.Parse(oldManifest, "default", true)
	newIndex := manifest.Parse(newManifest, "default", true)

	var buf bytes.Buffer
	changed := Manifests(oldIndex, newIndex, opts, &buf)
	require.True(t, changed)

	var entries []StructuredEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.True(t, entries[0].ChangesSuppressed)
	require.Empty(t, entries[0].Changes)
}

func findChange(changes []FieldChange, path, field string) (FieldChange, bool) {
	for _, change := range changes {
		if change.Path == path && change.Field == field {
			return change, true
		}
	}
	return FieldChange{}, false
}

func TestStructuredOutputErrorPaths(t *testing.T) {
	ansi.DisableColors(true)

	key := "default, failing, ConfigMap (v1)"
	opts := &Options{OutputFormat: "structured"}
	validConfigMap := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: valid
  namespace: default
data:
  foo: bar
`

	makeMapping := func(content string) *manifest.MappingResult {
		return &manifest.MappingResult{
			Name:    key,
			Kind:    "ConfigMap",
			Content: content,
		}
	}

	t.Run("InvalidNewManifestYAML", func(t *testing.T) {
		oldIndex := map[string]*manifest.MappingResult{
			key: makeMapping(validConfigMap),
		}
		newIndex := map[string]*manifest.MappingResult{
			key: makeMapping(":\n  not-valid: value"),
		}

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed, "Should report resource-level change even when YAML parsing fails")

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "MODIFY", entries[0].ChangeType)
	})

	t.Run("InvalidOldManifestYAML", func(t *testing.T) {
		oldIndex := map[string]*manifest.MappingResult{
			key: makeMapping("metadata:\n  name: invalid\n  namespace: default\n  labels:\n    : bad"),
		}
		newIndex := map[string]*manifest.MappingResult{
			key: makeMapping(validConfigMap),
		}

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed, "Should report resource-level change even when YAML parsing fails")

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "MODIFY", entries[0].ChangeType)
	})

	t.Run("ArrayDocumentProducesJSONUnmarshalError", func(t *testing.T) {
		listManifest := `
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: list
    namespace: default
  data:
    key: value
`
		newIndex := map[string]*manifest.MappingResult{
			key: makeMapping(listManifest),
		}

		var buf bytes.Buffer
		changed := Manifests(map[string]*manifest.MappingResult{}, newIndex, opts, &buf)
		require.True(t, changed, "Should report resource-level change even when JSON unmarshal fails")

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "ADD", entries[0].ChangeType)
	})
}

func TestStructuredOutputAdditionalScenarios(t *testing.T) {
	ansi.DisableColors(true)
	opts := &Options{OutputFormat: "structured"}

	t.Run("EmptyManifestHandling", func(t *testing.T) {
		emptyManifest := ``
		validManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: test
  namespace: default
`
		oldIndex := manifest.Parse(emptyManifest, "default", true)
		newIndex := manifest.Parse(validManifest, "default", true)

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed)

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "ADD", entries[0].ChangeType)
		require.False(t, entries[0].ResourceStatus.OldExists)
		require.True(t, entries[0].ResourceStatus.NewExists)
	})

	t.Run("NullYAMLDocument", func(t *testing.T) {
		nullManifest := `null`
		validManifest := `
apiVersion: v1
kind: Service
metadata:
  name: test
  namespace: default
`
		oldIndex := manifest.Parse(nullManifest, "default", true)
		newIndex := manifest.Parse(validManifest, "default", true)

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed)

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "ADD", entries[0].ChangeType)
	})

	t.Run("ComplexNestedStructuresForJSONPatch", func(t *testing.T) {
		oldManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: complex
  namespace: default
  labels:
    app: test
    version: v1
data:
  config.yaml: |
    nested:
      deeply:
        value: old
`
		newManifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: complex
  namespace: default
  labels:
    app: test
    version: v2
data:
  config.yaml: |
    nested:
      deeply:
        value: new
`
		oldIndex := manifest.Parse(oldManifest, "default", true)
		newIndex := manifest.Parse(newManifest, "default", true)

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed)

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "MODIFY", entries[0].ChangeType)
		require.NotEmpty(t, entries[0].Changes, "Should detect changes in nested structures")

		versionChange, found := findChange(entries[0].Changes, "metadata.labels", "version")
		require.True(t, found, "Should find version label change")
		require.Equal(t, "v1", versionChange.OldValue)
		require.Equal(t, "v2", versionChange.NewValue)
	})

	t.Run("ArrayChangesInStructuredOutput", func(t *testing.T) {
		oldManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: prod
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:v1
        env:
        - name: KEY1
          value: val1
`
		newManifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: prod
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:v2
        env:
        - name: KEY1
          value: val1
        - name: KEY2
          value: val2
`
		oldIndex := manifest.Parse(oldManifest, "prod", true)
		newIndex := manifest.Parse(newManifest, "prod", true)

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.True(t, changed)

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "MODIFY", entries[0].ChangeType)
		require.NotEmpty(t, entries[0].Changes, "Should detect changes")

		imageChange, found := findChange(entries[0].Changes, "spec.template.spec.containers[0]", "image")
		require.True(t, found, "Should find image change")
		require.Equal(t, "myapp:v1", imageChange.OldValue)
		require.Equal(t, "myapp:v2", imageChange.NewValue)
	})

	t.Run("BothManifestsEmpty", func(t *testing.T) {
		emptyManifest1 := ``
		emptyManifest2 := ``

		oldIndex := manifest.Parse(emptyManifest1, "default", true)
		newIndex := manifest.Parse(emptyManifest2, "default", true)

		var buf bytes.Buffer
		changed := Manifests(oldIndex, newIndex, opts, &buf)
		require.False(t, changed, "No changes should be detected with both empty")

		var entries []StructuredEntry
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
		require.Empty(t, entries, "Should have no entries for empty manifests")
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
		},
	}

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
		},
	}

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
		},
	}

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
		},
	}

	t.Run("OnChangeSecretWithByteData", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, false, false, []string{}, 0.5, []string{}} // NOTE: ShowSecrets = false

		if changesSeen := Manifests(specSecretWithByteData, specSecretWithByteDataChanged, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		// TODO: Why is there no empty line between the header and the start of the diff, like in the other diffs?
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
		diffOptions := Options{"diff", 10, false, false, false, []string{}, 0.5, []string{}} // NOTE: ShowSecrets = false

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
				Entries: []ReportEntry{
					{
						Diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{},
			expected: Report{
				Entries: []ReportEntry{
					{
						Diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
		},
		{
			name: "ignore all",
			input: Report{
				Entries: []ReportEntry{
					{
						Diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{".*world2?"},
			expected: Report{
				Entries: []ReportEntry{
					{
						Diffs: []difflib.DiffRecord{},
					},
				},
			},
		},
		{
			name: "ignore partial",
			input: Report{
				Entries: []ReportEntry{
					{
						Diffs: diffStrings("hello: world", "hello: world2", false),
					},
				},
			},
			supressRegex: []string{".*world2"},
			expected: Report{
				Entries: []ReportEntry{
					{
						Diffs: []difflib.DiffRecord{
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

func TestChangeOwnership(t *testing.T) {
	ansi.DisableColors(true)

	specOriginal := map[string]*manifest.MappingResult{
		"default, foobar, ConfigMap (v1)": {
			Name: "default, foobar, ConfigMap (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foobar
data:
  key1: value1
`,
		},
	}

	t.Run("OnChangeOwnershipWithoutSpecChange", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}} // NOTE: ShowSecrets = false

		newOwnedReleases := map[string]OwnershipDiff{
			"default, foobar, ConfigMap (v1)": {
				OldRelease: "default/oldfoobar",
				NewRelease: "default/foobar",
			},
		}
		if changesSeen := ManifestsOwnership(specOriginal, specOriginal, newOwnedReleases, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, foobar, ConfigMap (v1) changed ownership:
- default/oldfoobar
+ default/foobar
`, buf1.String())
	})

	t.Run("OnChangeOwnershipWithSpecChange", func(t *testing.T) {
		var buf1 bytes.Buffer
		diffOptions := Options{"diff", 10, false, true, false, []string{}, 0.5, []string{}} // NOTE: ShowSecrets = false

		specNew := map[string]*manifest.MappingResult{
			"default, foobar, ConfigMap (v1)": {
				Name: "default, foobar, ConfigMap (v1)",
				Kind: "Secret",
				Content: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: foobar
data:
  key1: newValue1
`,
			},
		}

		newOwnedReleases := map[string]OwnershipDiff{
			"default, foobar, ConfigMap (v1)": {
				OldRelease: "default/oldfoobar",
				NewRelease: "default/foobar",
			},
		}
		if changesSeen := ManifestsOwnership(specOriginal, specNew, newOwnedReleases, &diffOptions, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, foobar, ConfigMap (v1) changed ownership:
- default/oldfoobar
+ default/foobar
default, foobar, ConfigMap (v1) has changed:

  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: foobar
  data:
-   key1: value1
+   key1: newValue1

`, buf1.String())
	})
}

func TestDecodeSecrets(t *testing.T) {
	ansi.DisableColors(true)

	t.Run("decodeSecrets with valid base64 data", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: dmFsdWUx
  key2: dmFsdWUy
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: bmV3dmFsdWUx
  key2: dmFsdWUy
`,
		}
		decodeSecrets(old, new)
		require.Contains(t, old.Content, "key1: value1")
		require.Contains(t, old.Content, "key2: value2")
		require.Contains(t, new.Content, "key1: newvalue1")
		require.Contains(t, new.Content, "key2: value2")
	})

	t.Run("decodeSecrets with stringData", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1
  key2: value2
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1changed
  key2: value2
`,
		}
		decodeSecrets(old, new)
		require.Contains(t, old.Content, "key1: value1")
		require.Contains(t, old.Content, "key2: value2")
		require.Contains(t, new.Content, "key1: value1changed")
		require.Contains(t, new.Content, "key2: value2")
	})
	t.Run("decodeSecrets with stringData and data ensuring that stringData always precedes/overrides data on Secrets", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1.stringdata
  key2: value2.stringdata
data:
  key2: dmFsdWUyLmRhdGE=
  key3: dmFsdWUzLmRhdGE=
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1changed.stringdata
  key2: value2.stringdata
data:
  key3: dmFsdWUzLmRhdGE=
`,
		}
		decodeSecrets(old, new)
		require.Contains(t, old.Content, "key1: value1.stringdata")
		require.Contains(t, old.Content, "key2: value2.stringdata")
		require.Contains(t, old.Content, "key3: value3.data")
		require.Contains(t, new.Content, "key1: value1changed.stringdata")
		require.Contains(t, new.Content, "key2: value2.stringdata")
		require.Contains(t, new.Content, "key3: value3.data")
	})

	t.Run("decodeSecrets with invalid base64", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: invalidbase64
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: dmFsdWUx
`,
		}
		decodeSecrets(old, new)
		require.Contains(t, old.Content, "Error parsing old secret")
		require.Contains(t, new.Content, "key1: value1")
	})

	t.Run("decodeSecrets with non-Secret kind", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name:    "default, foo, ConfigMap (v1)",
			Kind:    "ConfigMap",
			Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}
		new := &manifest.MappingResult{
			Name:    "default, foo, ConfigMap (v1)",
			Kind:    "ConfigMap",
			Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}
		origOld := old.Content
		origNew := new.Content
		decodeSecrets(old, new)
		require.Equal(t, origOld, old.Content)
		require.Equal(t, origNew, new.Content)
	})

	t.Run("decodeSecrets with nil arguments", func(t *testing.T) {
		// Should not panic or change anything
		decodeSecrets(nil, nil)
	})
}

func TestRedactSecrets(t *testing.T) {
	ansi.DisableColors(true)

	t.Run("redactSecrets with valid base64 data", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: dmFsdWUx
  key2: dmFsdWUy
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: bmV3dmFsdWUx
  key2: dmFsdWUy
`,
		}
		redactSecrets(old, new)
		require.Contains(t, old.Content, "key1: '-------- # (6 bytes)'")
		require.Contains(t, old.Content, "key2: 'REDACTED # (6 bytes)'")
		require.Contains(t, new.Content, "key1: '++++++++ # (9 bytes)'")
		require.Contains(t, new.Content, "key2: 'REDACTED # (6 bytes)'")
	})

	t.Run("redactSecrets with stringData", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1
  key2: value2
`,
		}
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
stringData:
  key1: value1changed
  key2: value2
`,
		}
		redactSecrets(old, new)
		require.Contains(t, old.Content, "key1: '-------- # (6 bytes)'")
		require.Contains(t, old.Content, "key2: 'REDACTED # (6 bytes)'")
		require.Contains(t, new.Content, "key1: '++++++++ # (13 bytes)'")
		require.Contains(t, new.Content, "key2: 'REDACTED # (6 bytes)'")
	})

	t.Run("redactSecrets with nil arguments", func(t *testing.T) {
		// Should not panic or change anything
		redactSecrets(nil, nil)
	})

	t.Run("redactSecrets with non-Secret kind", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name:    "default, foo, ConfigMap (v1)",
			Kind:    "ConfigMap",
			Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}
		new := &manifest.MappingResult{
			Name:    "default, foo, ConfigMap (v1)",
			Kind:    "ConfigMap",
			Content: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: foo\n",
		}
		origOld := old.Content
		origNew := new.Content
		redactSecrets(old, new)
		require.Equal(t, origOld, old.Content)
		require.Equal(t, origNew, new.Content)
	})

	t.Run("redactSecrets with invalid YAML", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name:    "default, foo, Secret (v1)",
			Kind:    "Secret",
			Content: "invalid: yaml: :::",
		}
		new := &manifest.MappingResult{
			Name:    "default, foo, Secret (v1)",
			Kind:    "Secret",
			Content: "invalid: yaml: :::",
		}
		redactSecrets(old, new)
		require.Contains(t, old.Content, "Error parsing old secret")
		require.Contains(t, new.Content, "Error parsing new secret")
	})

	t.Run("redactSecrets with only old secret", func(t *testing.T) {
		old := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: dmFsdWUx
`,
		}
		redactSecrets(old, nil)
		require.Contains(t, old.Content, "key1: '-------- # (6 bytes)'")
	})

	t.Run("redactSecrets with only new secret", func(t *testing.T) {
		new := &manifest.MappingResult{
			Name: "default, foo, Secret (v1)",
			Kind: "Secret",
			Content: `
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: Opaque
data:
  key1: dmFsdWUx
`,
		}
		redactSecrets(nil, new)
		require.Contains(t, new.Content, "key1: '++++++++ # (6 bytes)'")
	})
}
