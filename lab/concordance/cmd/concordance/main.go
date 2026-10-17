// Command concordance is the instrument for one question: where a third party
// claims something about a subject we also hold a claim about, do we agree?
//
//	concordance --check                 # validate + prove no drift (exit 1 on failure)
//	concordance --render                # regenerate the artifacts from the records
//	concordance --stale [--as-of DATE]  # list records whose review_by has passed
//
// See lab/concordance/README.md for the policy on whose data is whose, and for
// exactly what is and is not checked.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/james-gibson/lab-safety/concordance/internal/concordance"
)

func main() {
	root := flag.String("root", ".", "the concordance root: the directory holding records/, sources/, schemas/ and the generated artifacts")
	repo := flag.String("repo", "../..", "the repo root, used to resolve our_claim.basis")
	check := flag.Bool("check", false, "validate every record and prove the generated artifacts have not drifted")
	render := flag.Bool("render", false, "regenerate concordance.csv, the README summary and concordance.feature from the records")
	stale := flag.Bool("stale", false, "list records whose review_by falls before --as-of (default: today)")
	asOf := flag.String("as-of", "", "date for --stale, YYYY-MM-DD (default: today)")
	flag.Parse()

	if !*check && !*render && !*stale {
		flag.Usage()
		os.Exit(2)
	}

	d, err := concordance.Open(*root, *repo)
	if err != nil {
		fail(err)
	}

	switch {
	case *render:
		fails, err := d.Render()
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			fmt.Fprintln(os.Stderr, "refused to render: fix the records first")
			os.Exit(1)
		}
		fmt.Println("rendered concordance.csv, README.md and concordance.feature from records/")
	case *stale:
		as, err := parseAsOf(*asOf)
		if err != nil {
			fail(err)
		}
		fails, err := d.Stale(as)
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Printf("no stale records as of %s\n", as.Format("2006-01-02"))
	default:
		fails, err := d.Check()
		if err != nil {
			fail(err)
		}
		if len(fails) > 0 {
			report(fails)
			os.Exit(1)
		}
		fmt.Println("check: every record is anchored to our own artefact; no drift")
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
	fmt.Fprintf(os.Stderr, "concordance: %v\n", err)
	os.Exit(2)
}
