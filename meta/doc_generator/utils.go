package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

const FlowPipelineRepo = "https://github.com/BelWue/flowpipeline"
const FlowPipelineFilesBase = FlowPipelineRepo + "/tree/master/"
const FlowPipelineCommitBase = FlowPipelineRepo + "/commit/"

func linkTo(display string, url string) string {
	if display == "" {
		display = url
	}
	return fmt.Sprintf("[%s](%s)", display, url)
}

func linkFromPath(path string, display string) string {
	projectBaseDir, err := projectRoot()
	if err != nil {
		log.Fatal().Err(err).Msg("Could not determine project base directory.")
		return display
	}

	targetFilePath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal().Err(err).Msgf("'%s' is not a valid path.", path)
		return display
	}

	if !strings.HasPrefix(targetFilePath, projectBaseDir) {
		log.Fatal().Msgf("The path '%s' is not within the project base directory '%s'.", targetFilePath, projectBaseDir)
		return display
	}

	return linkTo(display, FlowPipelineFilesBase+path)
}

func linkFromCommit(commit string) string {
	return linkTo(commit, FlowPipelineCommitBase+commit)
}

func projectRoot() (string, error) {
	projectBaseDir, err := filepath.Abs(envOr("PROJECT_BASE_DIR", "."))
	if err != nil {
		return "", err
	}
	return projectBaseDir, nil
}

func envOr(key string, defaultValue string) string {
	value, present := os.LookupEnv(key)
	if !present {
		return defaultValue
	}
	return value
}

func linkifyText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, " ", "-")
	return text
}

func unfilenamify(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "-", "")
	text = strings.ReplaceAll(text, "_", "")
	return text
}

func multiline(lines ...string) string {
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func summary(summary string, details string) string {
	return fmt.Sprintf(`<details>
<summary>%s</summary>

%s

</details>`, summary, details)
}
