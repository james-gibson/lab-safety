// Command survey is the model survey's tool: it projects profiles/*.json onto
// models.csv, the README summary and survey.feature, and proves none of the
// projections drifted.
//
//	survey --check                 # validate + prove no drift (exit 1 on failure)
//	survey --render                # regenerate the artifacts from the profiles
//	survey --stale [--as-of DATE]  # list profiles whose review_by has passed
//
// See lab/survey/README.md for the method, the schema, and exactly what is and
// is not checked.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/james-gibson/lab-safety/survey/internal/survey"
)

func main() {
	root := flag.String("root", ".", "survey root: the directory holding profiles/, schemas/ and the generated artifacts")
	check := flag.Bool("check", false, "validate every profile and prove models.csv, the README summary and survey.feature have not drifted")
	render := flag.Bool("render", false, "regenerate models.csv, the README summary and survey.feature from the profiles")
	stale := flag.Bool("stale", false, "list profiles whose review_by falls before --as-of (default: today)")
	asOf := flag.String("as-of", "", "date for --stale, YYYY-MM-DD (default: today)")
	flag.Parse()

	if !*check && !*render && !*stale {
		flag.Usage()
		os.Exit(2)
	}

	switch {
	case *render:
		fails, err := survey.Render(*root)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			fmt.Fprintln(os.Stderr, "refused to render: fix the profiles first")
			os.Exit(1)
		}
		fmt.Println("rendered models.csv, README.md and survey.feature from profiles/")
	case *stale:
		as, err := parseAsOf(*asOf)
		if err != nil {
			fail(err)
		}
		fails, err := survey.Stale(*root, as)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Printf("no stale profiles as of %s\n", as.Format("2006-01-02"))
	default:
		fails, err := survey.Check(*root)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Println("check: profiles are the source of truth; no drift")
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
