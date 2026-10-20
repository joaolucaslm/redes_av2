package app

import (
	"fmt"
	"io/fs"
	"os"

	fsmfiles "bancada/fsm"
	"bancada/internal/fsm"
	"bancada/internal/scenario"
	"bancada/internal/testbed"
	scenariofiles "bancada/scenarios"
)

const (
	EnvFSMDir        = "BANCADA_FSM_DIR"
	EnvScenarioDir   = "BANCADA_SCENARIOS_DIR"
	localFSMDir      = "fsm"
	localScenarioDir = "scenarios"
)

func pick(envVar, local string, embedded fs.FS) (fs.FS, string) {
	if dir := os.Getenv(envVar); dir != "" {
		return os.DirFS(dir), dir
	}
	if info, err := os.Stat(local); err == nil && info.IsDir() {
		return os.DirFS(local), local
	}
	return embedded, ""
}

type Sources struct {
	Catalog  *fsm.Catalog
	Specs    []scenario.Spec
	Warnings []string
}

func LoadSources() (Sources, error) {
	var src Sources
	fsmFS, fsmDir := pick(EnvFSMDir, localFSMDir, fsmfiles.Files)
	cat, err := testbed.LoadCatalog(fsmFS)
	if err != nil && fsmDir != "" {
		src.Warnings = append(src.Warnings, fmt.Sprintf("definições em %s rejeitadas; usando as embutidas: %v", fsmDir, err))
		cat, err = testbed.LoadCatalog(fsmfiles.Files)
	}
	if err != nil {
		return src, err
	}
	src.Catalog = cat

	specFS, specDir := pick(EnvScenarioDir, localScenarioDir, scenariofiles.Files)
	specs, err := scenario.LoadSpecs(specFS)
	if err != nil && specDir != "" {
		src.Warnings = append(src.Warnings, fmt.Sprintf("cenários em %s rejeitados; usando os embutidos: %v", specDir, err))
		specs, err = scenario.LoadSpecs(scenariofiles.Files)
	}
	if err != nil {
		return src, err
	}
	src.Specs = specs
	return src, nil
}
