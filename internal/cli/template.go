package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func fillPRTemplate(client *gh.Client, templateName string) (string, error) {
	tmpl, err := client.GetPRTemplate(templateName)
	if err != nil || tmpl == nil {
		return "", nil
	}

	fmt.Println()
	fmt.Println(bold.Render("  Pool requirements"))
	printDim("  " + strings.Repeat("─", 36))
	fmt.Println()

	rendered := renderTemplateInfo(tmpl)
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) != "" {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()

	if !tmpl.HasFields() {
		return tmpl.Raw, nil
	}

	reader := bufio.NewReader(os.Stdin)
	values := make(map[string]string)

	for _, field := range tmpl.Fields {
		value := prompt(reader, fmt.Sprintf("  %s: ", field.Label))
		if field.Required && value == "" {
			printError(fmt.Sprintf("%s is required", field.Label))
			return "", fmt.Errorf("missing required field: %s", field.Name)
		}
		values[field.Name] = value
	}

	return tmpl.Render(values), nil
}

func renderTemplateInfo(tmpl *gh.PRTemplate) string {
	lines := strings.Split(tmpl.Raw, "\n")
	var display []string

	for _, line := range lines {
		if !gh.IsFieldPlaceholder(line) {
			display = append(display, line)
		}
	}

	return strings.Join(display, "\n")
}
