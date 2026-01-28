package diff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"text/template"

	"github.com/aryann/difflib"
	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"github.com/mgutz/ansi"
)

// Report to store report data and format
type Report struct {
	format  ReportFormat
	Entries []ReportEntry
}

// ReportEntry to store changes between releases
type ReportEntry struct {
	Key             string
	SuppressedKinds []string
	Kind            string
	Context         int
	Diffs           []difflib.DiffRecord
	ChangeType      string
}

// ReportFormat to the context to make a changes report
type ReportFormat struct {
	output       func(r *Report, to io.Writer)
	changestyles map[string]ChangeStyle
}

// ChangeStyle for styling the report
type ChangeStyle struct {
	color   string
	message string
}

// ReportTemplateSpec for common template spec
type ReportTemplateSpec struct {
	Namespace string
	Name      string
	Kind      string
	API       string
	Change    string
}

// setupReportFormat: process output argument.
func (r *Report) setupReportFormat(format string) {
	switch format {
	case "simple":
		setupSimpleReport(r)
	case "template":
		setupTemplateReport(r)
	case "json":
		setupJSONReport(r)
	case "dyff":
		setupDyffReport(r)
	case "ai":
		setupAIReport(r)
	default:
		setupDiffReport(r)
	}
}

func setupDyffReport(r *Report) {
	r.format.output = printDyffReport
}

func setupAIReport(r *Report) {
	r.format.output = printAIReport
}

func printDyffReport(r *Report, to io.Writer) {
	currentFile, _ := os.CreateTemp("", "existing-values")
	defer func() {
		_ = os.Remove(currentFile.Name())
	}()
	newFile, _ := os.CreateTemp("", "new-values")
	defer func() {
		_ = os.Remove(newFile.Name())
	}()

	for _, entry := range r.Entries {
		_, _ = currentFile.WriteString("---\n")
		_, _ = newFile.WriteString("---\n")
		for _, record := range entry.Diffs {
			switch record.Delta {
			case difflib.Common:
				_, _ = currentFile.WriteString(record.Payload + "\n")
				_, _ = newFile.WriteString(record.Payload + "\n")
			case difflib.LeftOnly:
				_, _ = currentFile.WriteString(record.Payload + "\n")
			case difflib.RightOnly:
				_, _ = newFile.WriteString(record.Payload + "\n")
			}
		}
	}
	_ = currentFile.Close()
	_ = newFile.Close()

	currentInputFile, newInputFile, _ := ytbx.LoadFiles(currentFile.Name(), newFile.Name())

	report, _ := dyff.CompareInputFiles(currentInputFile, newInputFile)
	reportWriter := &dyff.HumanReport{
		Report:               report,
		OmitHeader:           true,
		MinorChangeThreshold: 0.1,
	}
	_ = reportWriter.WriteReport(to)
}

func printAIReport(r *Report, to io.Writer) {
	_, _ = fmt.Fprint(to, "[\n")
	for i, entry := range r.Entries {
		templateData := ReportTemplateSpec{}
		err := templateData.loadFromKey(entry.Key)
		if err != nil {
			log.Println("error processing report entry")
			continue
		}

		_, _ = fmt.Fprintf(to, "  {\n")
		_, _ = fmt.Fprintf(to, "    \"api\": \"%s\",\n", escapeJSON(templateData.API))
		_, _ = fmt.Fprintf(to, "    \"kind\": \"%s\",\n", escapeJSON(templateData.Kind))
		_, _ = fmt.Fprintf(to, "    \"namespace\": \"%s\",\n", escapeJSON(templateData.Namespace))
		_, _ = fmt.Fprintf(to, "    \"name\": \"%s\",\n", escapeJSON(templateData.Name))
		_, _ = fmt.Fprintf(to, "    \"change\": \"%s\",\n", escapeJSON(entry.ChangeType))
		_, _ = fmt.Fprintf(to, "    \"diffs\": [\n")

		for j, record := range entry.Diffs {
			deltaType := "common"
			switch record.Delta {
			case difflib.LeftOnly:
				deltaType = "removed"
			case difflib.RightOnly:
				deltaType = "added"
			}

			_, _ = fmt.Fprintf(to, "      {\n")
			_, _ = fmt.Fprintf(to, "        \"type\": \"%s\",\n", deltaType)
			_, _ = fmt.Fprintf(to, "        \"content\": %s\n", escapeJSONString(record.Payload))
			if j < len(entry.Diffs)-1 {
				_, _ = fmt.Fprint(to, "      },\n")
			} else {
				_, _ = fmt.Fprint(to, "      }\n")
			}
		}

		if i < len(r.Entries)-1 {
			_, _ = fmt.Fprintf(to, "    ]\n  },\n")
		} else {
			_, _ = fmt.Fprintf(to, "    ]\n  }\n")
		}
	}
	_, _ = fmt.Fprint(to, "]\n")
}

func escapeJSON(s string) string {
	if s == "" {
		return ""
	}
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func escapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// addEntry: stores diff changes.
func (r *Report) addEntry(key string, suppressedKinds []string, kind string, context int, diffs []difflib.DiffRecord, changeType string) {
	entry := ReportEntry{
		key,
		suppressedKinds,
		kind,
		context,
		diffs,
		changeType,
	}
	r.Entries = append(r.Entries, entry)
}

// print: prints entries added to the report.
func (r *Report) print(to io.Writer) {
	r.format.output(r, to)
}

// clean: needed for testing
func (r *Report) clean() {
	r.Entries = nil
}

// setup report for default output: diff
func setupDiffReport(r *Report) {
	r.format.output = printDiffReport
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: "has been added:"}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: "has been removed:"}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: "has changed:"}
	r.format.changestyles["OWNERSHIP"] = ChangeStyle{color: "magenta", message: "changed ownership:"}
	r.format.changestyles["MODIFY_SUPPRESSED"] = ChangeStyle{color: "blue+h", message: "has changed, but diff is empty after suppression."}
}

// print report for default output: diff
func printDiffReport(r *Report, to io.Writer) {
	for _, entry := range r.Entries {
		_, _ = fmt.Fprintf(
			to,
			ansi.Color("%s %s", r.format.changestyles[entry.ChangeType].color)+"\n",
			entry.Key,
			r.format.changestyles[entry.ChangeType].message,
		)
		printDiffRecords(entry.SuppressedKinds, entry.Kind, entry.Context, entry.Diffs, to)
	}
}

// setup report for simple output.
func setupSimpleReport(r *Report) {
	r.format.output = printSimpleReport
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: "to be added."}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: "to be removed."}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: "to be changed."}
	r.format.changestyles["OWNERSHIP"] = ChangeStyle{color: "magenta", message: "to change ownership."}
	r.format.changestyles["MODIFY_SUPPRESSED"] = ChangeStyle{color: "blue+h", message: "has changed, but diff is empty after suppression."}
}

// print report for simple output
func printSimpleReport(r *Report, to io.Writer) {
	summary := map[string]int{
		"ADD":               0,
		"REMOVE":            0,
		"MODIFY":            0,
		"OWNERSHIP":         0,
		"MODIFY_SUPPRESSED": 0,
	}
	for _, entry := range r.Entries {
		_, _ = fmt.Fprintf(to, ansi.Color("%s %s", r.format.changestyles[entry.ChangeType].color)+"\n",
			entry.Key,
			r.format.changestyles[entry.ChangeType].message,
		)
		summary[entry.ChangeType]++
	}
	_, _ = fmt.Fprintf(to, "Plan: %d to add, %d to change, %d to destroy, %d to change ownership.\n", summary["ADD"], summary["MODIFY"], summary["REMOVE"], summary["OWNERSHIP"])
}

func newTemplate(name string) *template.Template {
	// Prepare template functions
	funcsMap := template.FuncMap{
		"last": func(x int, a interface{}) bool {
			return x == reflect.ValueOf(a).Len()-1
		},
	}

	return template.New(name).Funcs(funcsMap)
}

// setup report for json output
func setupJSONReport(r *Report) {
	t, err := newTemplate("entries").Parse(defaultTemplateReport)
	if err != nil {
		log.Fatalf("Error loading default template: %v", err)
	}

	r.format.output = templateReportPrinter(t)
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: ""}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: ""}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: ""}
	r.format.changestyles["OWNERSHIP"] = ChangeStyle{color: "magenta", message: ""}
	r.format.changestyles["MODIFY_SUPPRESSED"] = ChangeStyle{color: "blue+h", message: ""}
}

// setup report for template output
func setupTemplateReport(r *Report) {
	var tpl *template.Template

	{
		tplFile, present := os.LookupEnv("HELM_DIFF_TPL")
		if present {
			t, err := newTemplate(filepath.Base(tplFile)).ParseFiles(tplFile)
			if err != nil {
				fmt.Println(err)
				log.Fatalf("Error loading custom template")
			}
			tpl = t
		} else {
			// Render
			t, err := newTemplate("entries").Parse(defaultTemplateReport)
			if err != nil {
				log.Fatalf("Error loading default template")
			}
			tpl = t
		}
	}

	r.format.output = templateReportPrinter(tpl)
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: ""}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: ""}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: ""}
	r.format.changestyles["OWNERSHIP"] = ChangeStyle{color: "magenta", message: ""}
	r.format.changestyles["MODIFY_SUPPRESSED"] = ChangeStyle{color: "blue+h", message: ""}
}

// report with template output will only have access to ReportTemplateSpec.
// This function reverts parsedMetadata.String()
func (t *ReportTemplateSpec) loadFromKey(key string) error {
	pattern := regexp.MustCompile(`(?P<namespace>[a-z0-9-]+), (?P<name>[a-z0-9.-]+), (?P<kind>\w+) \((?P<api>[^)]+)\)`)
	matches := pattern.FindStringSubmatch(key)
	if len(matches) > 1 {
		t.Namespace = matches[1]
		t.Name = matches[2]
		t.Kind = matches[3]
		t.API = matches[4]
		return nil
	}
	return errors.New("key string didn't match regexp")
}

// load and print report for template output
func templateReportPrinter(t *template.Template) func(r *Report, to io.Writer) {
	return func(r *Report, to io.Writer) {
		var templateDataArray []ReportTemplateSpec

		for _, entry := range r.Entries {
			templateData := ReportTemplateSpec{}
			err := templateData.loadFromKey(entry.Key)
			if err != nil {
				log.Println("error processing report entry")
			} else {
				templateData.Change = entry.ChangeType
				templateDataArray = append(templateDataArray, templateData)
			}
		}

		_ = t.Execute(to, templateDataArray)
		_, _ = to.Write([]byte("\n"))
	}
}
