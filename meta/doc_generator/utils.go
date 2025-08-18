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

func linkFromPath(path string) (string, error) {
	projectBaseDir, err := projectRoot()
	if err != nil {
		log.Fatal().Err(err).Msg("Could not determine project base directory.")
		return "", err
	}

	targetFilePath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal().Err(err).Msgf("'%s' is not a valid path.", path)
		return "", err
	}

	if !strings.HasPrefix(targetFilePath, projectBaseDir) {
		log.Fatal().Msgf("The path '%s' is not within the project base directory '%s'.", targetFilePath, projectBaseDir)
		return "", err
	}

	return fmt.Sprintf("[%s](%s)", path, FlowPipelineFilesBase+path), nil
}

func linkFromCommit(commit string) string {
	return fmt.Sprintf("[%s](%s)", commit, FlowPipelineCommitBase+commit)
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
