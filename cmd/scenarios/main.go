package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	fsmfiles "bancada/fsm"
	"bancada/internal/scenario"
	"bancada/internal/testbed"
	scenariofiles "bancada/scenarios"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func source(dir string, embedded fs.FS) fs.FS {
	if dir == "" {
		return embedded
	}
	return os.DirFS(dir)
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("scenarios", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scenarioDir := flags.String("scenarios", "", "diretório com cenários JSON (padrão: os embutidos no binário)")
	fsmDir := flags.String("fsm", "", "diretório com definições de FSM (padrão: as embutidas no binário)")
	outDir := flags.String("out", "report", "diretório onde gravar scenarios.json e scenarios.md")
	only := flags.String("only", "", "ids de cenários separados por vírgula")
	trace := flags.String("trace", "", "imprime o log completo de um cenário: id ou id:execução")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cat, err := testbed.LoadCatalog(source(*fsmDir, fsmfiles.Files))
	if err != nil {
		fmt.Fprintf(stderr, "definições de FSM inválidas:\n%v\n", err)
		return 2
	}
	specs, err := scenario.LoadSpecs(source(*scenarioDir, scenariofiles.Files))
	if err != nil {
		fmt.Fprintf(stderr, "cenários inválidos:\n%v\n", err)
		return 2
	}
	runner := scenario.Runner{Catalog: cat}

	if *trace != "" {
		id, runName, _ := strings.Cut(*trace, ":")
		spec, err := scenario.Find(specs, id)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result, err := runner.Trace(spec, runName)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		scenario.WriteTrace(stdout, result)
		return 0
	}

	if *only != "" {
		wanted := strings.Split(*only, ",")
		specs = slices.DeleteFunc(specs, func(s scenario.Spec) bool { return !slices.Contains(wanted, s.ID) })
	}

	results := runner.RunAll(specs)
	if err := scenario.WriteTable(stdout, results); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	scenario.WriteFailures(stdout, results)
	if err := writeReports(*outDir, results); err != nil {
		fmt.Fprintf(stderr, "não foi possível gravar o relatório: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "relatório gravado em %s\n", filepath.Join(*outDir, "scenarios.{json,md}"))
	for _, r := range results {
		if !r.Passed {
			return 1
		}
	}
	return 0
}

func writeReports(dir string, results []scenario.Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(dir, "scenarios.json"))
	if err != nil {
		return err
	}
	jsonErr := scenario.WriteJSON(jsonFile, results)
	mdFile, err := os.Create(filepath.Join(dir, "scenarios.md"))
	if err != nil {
		return errors.Join(jsonErr, jsonFile.Close(), err)
	}
	mdErr := scenario.WriteMarkdown(mdFile, results)
	return errors.Join(jsonErr, mdErr, jsonFile.Close(), mdFile.Close())
}
