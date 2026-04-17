package diff

import (
	"bytes"
	"testing"

	"github.com/aryann/difflib"
	"github.com/stretchr/testify/require"
)

func TestLoadFromKey(t *testing.T) {
	keyToReportTemplateSpec := map[string]ReportTemplateSpec{
		"default, nginx, Deployment (apps)": {
			Namespace: "default",
			Name:      "nginx",
			Kind:      "Deployment",
			API:       "apps",
		},
		"default, probes.monitoring.coreos.com, CustomResourceDefinition (apiextensions.k8s.io)": {
			Namespace: "default",
			Name:      "probes.monitoring.coreos.com",
			Kind:      "CustomResourceDefinition",
			API:       "apiextensions.k8s.io",
		},
		"default, my-cert, Certificate (cert-manager.io/v1)": {
			Namespace: "default",
			Name:      "my-cert",
			Kind:      "Certificate",
			API:       "cert-manager.io/v1",
		},
	}

	for key, expectedTemplateSpec := range keyToReportTemplateSpec {
		templateSpec := &ReportTemplateSpec{}
		require.NoError(t, templateSpec.loadFromKey(key))
		require.Equal(t, expectedTemplateSpec, *templateSpec)
	}
}

func TestPrintDyffReport(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "MODIFY",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.Common},
					{Payload: "kind: Deployment", Delta: difflib.Common},
					{Payload: "metadata:", Delta: difflib.Common},
					{Payload: "  name: nginx", Delta: difflib.Common},
					{Payload: "spec:", Delta: difflib.Common},
					{Payload: "  replicas: 2", Delta: difflib.LeftOnly},
					{Payload: "  replicas: 3", Delta: difflib.RightOnly},
				},
			},
		},
	}

	var buf bytes.Buffer
	printDyffReport(report, &buf)

	output := buf.String()
	require.NotEmpty(t, output)
	require.Contains(t, output, "replicas", "Expected dyff output to mention replicas field")
	require.Contains(t, output, "- 2", "Expected dyff output to show original replicas value")
	require.Contains(t, output, "+ 3", "Expected dyff output to show updated replicas value")
}

func TestPrintDyffReportWithAddAndRemove(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			{
				Key:        "default, old-app, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "REMOVE",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.LeftOnly},
					{Payload: "kind: Deployment", Delta: difflib.LeftOnly},
					{Payload: "metadata:", Delta: difflib.LeftOnly},
					{Payload: "  name: old-app", Delta: difflib.LeftOnly},
				},
			},
			{
				Key:        "default, new-app, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "ADD",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.RightOnly},
					{Payload: "kind: Deployment", Delta: difflib.RightOnly},
					{Payload: "metadata:", Delta: difflib.RightOnly},
					{Payload: "  name: new-app", Delta: difflib.RightOnly},
				},
			},
		},
	}

	var buf bytes.Buffer
	printDyffReport(report, &buf)

	output := buf.String()
	require.NotEmpty(t, output)
	require.Contains(t, output, "old-app", "Expected dyff output to show removed resource old-app")
	require.Contains(t, output, "new-app", "Expected dyff output to show added resource new-app")
}

func TestPrintDyffReportDoesNotMergeAddRemoveWithFindRenames(t *testing.T) {
	addRemoveReport := &Report{
		FindRenames: 1.0,
		Entries: []ReportEntry{
			{
				Key:        "default, old-app, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "REMOVE",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.LeftOnly},
					{Payload: "kind: Deployment", Delta: difflib.LeftOnly},
					{Payload: "metadata:", Delta: difflib.LeftOnly},
					{Payload: "  name: old-app", Delta: difflib.LeftOnly},
				},
			},
			{
				Key:        "default, new-app, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "ADD",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.RightOnly},
					{Payload: "kind: Deployment", Delta: difflib.RightOnly},
					{Payload: "metadata:", Delta: difflib.RightOnly},
					{Payload: "  name: new-app", Delta: difflib.RightOnly},
				},
			},
		},
	}

	modifyReport := &Report{
		FindRenames: 1.0,
		Entries: []ReportEntry{
			{
				Key:        "default, app, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "MODIFY",
				Diffs: []difflib.DiffRecord{
					{Payload: "apiVersion: apps/v1", Delta: difflib.Common},
					{Payload: "kind: Deployment", Delta: difflib.Common},
					{Payload: "metadata:", Delta: difflib.Common},
					{Payload: "  name: app", Delta: difflib.Common},
					{Payload: "  name: old-app", Delta: difflib.LeftOnly},
					{Payload: "  name: new-app", Delta: difflib.RightOnly},
				},
			},
		},
	}

	var addRemoveBuf bytes.Buffer
	printDyffReport(addRemoveReport, &addRemoveBuf)
	addRemoveOutput := addRemoveBuf.String()

	var modifyBuf bytes.Buffer
	printDyffReport(modifyReport, &modifyBuf)
	modifyOutput := modifyBuf.String()

	require.NotEqual(t, addRemoveOutput, modifyOutput,
		"ADD+REMOVE output should differ from MODIFY output to verify dyff does not merge them as a rename")
	require.Contains(t, addRemoveOutput, "old-app")
	require.Contains(t, addRemoveOutput, "new-app")
}

func TestPrintDyffReportFindRenamesChangesOutput(t *testing.T) {
	entries := []ReportEntry{
		{
			Key:        "default, old-app, Deployment (apps)",
			Kind:       "Deployment",
			ChangeType: "REMOVE",
			Diffs: []difflib.DiffRecord{
				{Payload: "apiVersion: apps/v1", Delta: difflib.LeftOnly},
				{Payload: "kind: Deployment", Delta: difflib.LeftOnly},
				{Payload: "metadata:", Delta: difflib.LeftOnly},
				{Payload: "  name: old-app", Delta: difflib.LeftOnly},
				{Payload: "spec:", Delta: difflib.LeftOnly},
				{Payload: "  replicas: 3", Delta: difflib.LeftOnly},
			},
		},
		{
			Key:        "default, new-app, Deployment (apps)",
			Kind:       "Deployment",
			ChangeType: "ADD",
			Diffs: []difflib.DiffRecord{
				{Payload: "apiVersion: apps/v1", Delta: difflib.RightOnly},
				{Payload: "kind: Deployment", Delta: difflib.RightOnly},
				{Payload: "metadata:", Delta: difflib.RightOnly},
				{Payload: "  name: new-app", Delta: difflib.RightOnly},
				{Payload: "spec:", Delta: difflib.RightOnly},
				{Payload: "  replicas: 3", Delta: difflib.RightOnly},
			},
		},
	}

	var noRenameBuf bytes.Buffer
	printDyffReport(&Report{FindRenames: 0, Entries: entries}, &noRenameBuf)
	noRenameOutput := noRenameBuf.String()

	var withRenameBuf bytes.Buffer
	printDyffReport(&Report{FindRenames: 1.0, Entries: entries}, &withRenameBuf)
	withRenameOutput := withRenameBuf.String()

	require.NotEqual(t, noRenameOutput, withRenameOutput,
		"Output with FindRenames > 0 should differ from FindRenames == 0 for similar ADD+REMOVE entries")
}

func TestPrintDyffReportEmpty(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{},
	}

	var buf bytes.Buffer
	printDyffReport(report, &buf)

	output := buf.String()
	require.Equal(t, "\n", output)
}
