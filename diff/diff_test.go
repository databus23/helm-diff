package diff

import (
	"bytes"
	"os"
	"testing"

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

	t.Run("OnChange", func(t *testing.T) {

		var buf1 bytes.Buffer

		if changesSeen := Manifests(specBeta, specRelease, []string{}, true, 10, "diff", false, &buf1); !changesSeen {
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

	t.Run("OnNoChange", func(t *testing.T) {
		var buf2 bytes.Buffer

		if changesSeen := Manifests(specRelease, specRelease, []string{}, true, 10, "diff", false, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, ``, buf2.String())
	})

	t.Run("OnChangeSimple", func(t *testing.T) {

		var buf1 bytes.Buffer

		if changesSeen := Manifests(specBeta, specRelease, []string{}, true, 10, "simple", false, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `true` to indicate that it has seen any change(s), but was `false`")
		}

		require.Equal(t, `default, nginx, Deployment (apps) to be changed.
Plan: 0 to add, 1 to change, 0 to destroy.
`, buf1.String())
	})

	t.Run("OnNoChangeSimple", func(t *testing.T) {
		var buf2 bytes.Buffer

		if changesSeen := Manifests(specRelease, specRelease, []string{}, true, 10, "simple", false, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Plan: 0 to add, 0 to change, 0 to destroy.\n", buf2.String())
	})

	t.Run("OnChangeTemplate", func(t *testing.T) {

		var buf1 bytes.Buffer

		if changesSeen := Manifests(specBeta, specRelease, []string{}, true, 10, "template", false, &buf1); !changesSeen {
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

		if changesSeen := Manifests(specBeta, specRelease, []string{}, true, 10, "json", false, &buf1); !changesSeen {
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

		if changesSeen := Manifests(specRelease, specRelease, []string{}, true, 10, "template", false, &buf2); changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "[]\n", buf2.String())
	})

	t.Run("OnChangeCustomTemplate", func(t *testing.T) {
		var buf1 bytes.Buffer
		os.Setenv("HELM_DIFF_TPL", "testdata/customTemplate.tpl")
		if changesSeen := Manifests(specBeta, specRelease, []string{}, true, 10, "template", false, &buf1); !changesSeen {
			t.Error("Unexpected return value from Manifests: Expected the return value to be `false` to indicate that it has NOT seen any change(s), but was `true`")
		}

		require.Equal(t, "Resource name: nginx\n", buf1.String())
	})
}
