// Command survey is the survey tool: it projects profiles/*.json onto the
// dataset's CSV, README summary and Gherkin, and proves none of the projections
// drifted.
//
//	survey --check                 # validate + prove no drift (exit 1 on failure)
//	survey --render                # regenerate the artifacts from the profiles
//	survey --stale [--as-of DATE]  # list profiles whose review_by has passed
//
// --dataset model (default) reads lab/survey; --dataset tool reads
// lab/survey/tool. See each dataset's README.md for its method, schema and
// exactly what is and is not checked.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/james-gibson/lab-safety/survey/internal/survey"
)

func main() {
	dataset := flag.String("dataset", "model", "survey dataset: model | tool")
	root := flag.String("root", "", "survey root (default: . for model, tool for tool)")
	check := flag.Bool("check", false, "validate every profile and prove the generated artifacts have not drifted")
	render := flag.Bool("render", false, "regenerate the CSV, the README summary and the feature from the profiles")
	stale := flag.Bool("stale", false, "list profiles whose review_by falls before --as-of (default: today)")
	asOf := flag.String("as-of", "", "date for --stale, YYYY-MM-DD (default: today)")
	flag.Parse()

	var ds *survey.Dataset
	switch *dataset {
	case "model":
		ds = survey.Model()
	case "tool":
		ds = survey.Tool()
	default:
		fail(fmt.Errorf("unknown --dataset %q (want model or tool)", *dataset))
	}
	r := *root
	if r == "" {
		r = ds.Root
	}

	if !*check && !*render && !*stale {
		flag.Usage()
		os.Exit(2)
	}

	switch {
	case *render:
		fails, err := survey.RenderDataset(ds, r)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			fmt.Fprintln(os.Stderr, "refused to render: fix the profiles first")
			os.Exit(1)
		}
		fmt.Printf("rendered %s, %s and %s from profiles/\n", ds.CSVPath, ds.ReadmePath, ds.FeaturePath)
	case *stale:
		as, err := parseAsOf(*asOf)
		if err != nil {
			fail(err)
		}
		fails, err := survey.StaleDataset(ds, r, as)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Printf("no stale profiles as of %s\n", as.Format("2006-01-02"))
	default:
		fails, err := survey.CheckDataset(ds, r)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Printf("check: %s profiles are the source of truth; no drift\n", ds.Name)
	}
}

func parseAsOf(s string) (time.Time, error) {
	if s == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("--as-of %q is not an ISO date (YYYY-MM-DD)", s)
	}
	return t, nil
}

func report(fails []string) {
	for _, f := range fails {
		fmt.Fprintf(os.Stderr, "FAIL: %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "%d check(s) failed\n", len(fails))
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "survey: %v\n", err)
	os.Exit(2)
}
