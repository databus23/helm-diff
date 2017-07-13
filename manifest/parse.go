package manifest

import (
	"bufio"
	"bytes"
	"strings"
)

var yamlSeperator = []byte("---\n# Source: ")

func scanYamlSpecs(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, yamlSeperator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(yamlSeperator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func splitSpec(token string) (string, string) {
	if i := strings.Index(token, "\n"); i >= 0 {
		return token[0:i], token[i+1:]
	}
	return "", ""
}

func Parse(manifest string) map[string]string {
	scanner := bufio.NewScanner(strings.NewReader(manifest))
	scanner.Split(scanYamlSpecs)
	//Allow for tokens (specs) up to 1M in size
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 1048576)
	//Discard the first result, we only care about everything after the first seperator
	scanner.Scan()

	result := make(map[string]string)

	for scanner.Scan() {
		source, content := splitSpec(scanner.Text())
		//Since helm 2.5.0 the '# Source:' stanze appears multiple times per template (for each yaml doc)
		if _, ok := result[source]; ok {
			result[source] = result[source] + "\n" + content
		} else {
			result[source] = content
		}
	}
	return result

}
