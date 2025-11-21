package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	startMarker = "<!-- BEGIN_MAKEFILE_DOCS -->"
	endMarker   = "<!-- END_MAKEFILE_DOCS -->"
)

func main() {
	makefile, err := os.Open("Makefile")
	if err != nil {
		panic(err)
	}
	defer makefile.Close()

	var targets []string
	scanner := bufio.NewScanner(makefile)
	re := regexp.MustCompile(`^([a-zA-Z0-9_-]+):.*?## (.*)$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			target := matches[1]
			desc := matches[2]
			targets = append(targets, fmt.Sprintf("| `%s` | %s |", target, desc))
		}
	}

	sort.Strings(targets)

	readmeContent, err := os.ReadFile("README.md")
	if err != nil {
		panic(err)
	}
	content := string(readmeContent)

	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 {
		fmt.Println("Markers not found in README.md")
		return
	}

	newContent := content[:startIdx+len(startMarker)] + "\n| Target | Description |\n|---|---|\n" + strings.Join(targets, "\n") + "\n" + content[endIdx:]

	err = os.WriteFile("README.md", []byte(newContent), 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("README.md updated successfully")
}
