package diff

import (
	"errors"
	"fmt"
	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"text/template"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"
)

// Report to store report data and format
type Report struct {
	format  ReportFormat
	entries []ReportEntry
}

// ReportEntry to store changes between releases
type ReportEntry struct {
	key             string
	suppressedKinds []string
	kind            string
	context         int
	diffs           []difflib.DiffRecord
	changeType      string
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
	default:
		setupDiffReport(r)
	}
}

func setupDyffReport(r *Report) {
	r.format.output = printDyffReport
}

func printDyffReport(r *Report, to io.Writer) {
	currentFile, _ := os.CreateTemp("", "existing-values")
	defer os.Remove(currentFile.Name())
	newFile, _ := os.CreateTemp("", "new-values")
	defer os.Remove(newFile.Name())

	for _, entry := range r.entries {
		_, _ = currentFile.WriteString("---\n")
		_, _ = newFile.WriteString("---\n")
		for _, record := range entry.diffs {
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
	r.entries = append(r.entries, entry)
}

// print: prints entries added to the report.
func (r *Report) print(to io.Writer) {
	r.format.output(r, to)
}

// clean: needed for testing
func (r *Report) clean() {
	r.entries = nil
}

// setup report for default output: diff
func setupDiffReport(r *Report) {
	r.format.output = printDiffReport
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: "has been added:"}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: "has been removed:"}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: "has changed:"}
}

// print report for default output: diff
func printDiffReport(r *Report, to io.Writer) {
	for _, entry := range r.entries {
		fmt.Fprintf(to, ansi.Color("%s %s", "yellow")+"\n", entry.key, r.format.changestyles[entry.changeType].message)
		printDiffRecords(entry.suppressedKinds, entry.kind, entry.context, entry.diffs, to)
	}

}

// setup report for simple output.
func setupSimpleReport(r *Report) {
	r.format.output = printSimpleReport
	r.format.changestyles = make(map[string]ChangeStyle)
	r.format.changestyles["ADD"] = ChangeStyle{color: "green", message: "to be added."}
	r.format.changestyles["REMOVE"] = ChangeStyle{color: "red", message: "to be removed."}
	r.format.changestyles["MODIFY"] = ChangeStyle{color: "yellow", message: "to be changed."}
}

// print report for simple output
func printSimpleReport(r *Report, to io.Writer) {
	var summary = map[string]int{
		"ADD":    0,
		"REMOVE": 0,
		"MODIFY": 0,
	}
	for _, entry := range r.entries {
		fmt.Fprintf(to, ansi.Color("%s %s", r.format.changestyles[entry.changeType].color)+"\n",
			entry.key,
			r.format.changestyles[entry.changeType].message,
		)
		summary[entry.changeType]++
	}
	fmt.Fprintf(to, "Plan: %d to add, %d to change, %d to destroy.\n", summary["ADD"], summary["MODIFY"], summary["REMOVE"])
}

func newTemplate(name string) *template.Template {
	// Prepare template functions
	var funcsMap = template.FuncMap{
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

		for _, entry := range r.entries {
			templateData := ReportTemplateSpec{}
			err := templateData.loadFromKey(entry.key)
			if err != nil {
				log.Println("error processing report entry")
			} else {
				templateData.Change = entry.changeType
				templateDataArray = append(templateDataArray, templateData)
			}
		}

		t.Execute(to, templateDataArray)
		_, _ = to.Write([]byte("\n"))
	}
}
