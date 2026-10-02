package cmd

import (
	"reflect"
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/output"
)

func TestSelectedFieldsTrimAndDeduplicate(t *testing.T) {
	previous := selectFlag
	selectFlag = " id, from.address, id ,subject "
	t.Cleanup(func() { selectFlag = previous })

	want := []string{"id", "from.address", "subject"}
	if got := selectedFields(); !reflect.DeepEqual(got, want) {
		t.Fatalf("selected fields = %#v, want %#v", got, want)
	}
}

func TestAutomationOutputFlagValidation(t *testing.T) {
	previousJSON, previousQuiet := jsonFlag, quietFlag
	previousResultsOnly, previousIDsOnly := resultsOnlyFlag, idsOnlyFlag
	previousCount, previousSelect := countFlag, selectFlag
	t.Cleanup(func() {
		jsonFlag, quietFlag = previousJSON, previousQuiet
		resultsOnlyFlag, idsOnlyFlag = previousResultsOnly, previousIDsOnly
		countFlag, selectFlag = previousCount, previousSelect
	})

	jsonFlag, quietFlag, resultsOnlyFlag, idsOnlyFlag, countFlag = false, false, true, false, false
	selectFlag = "id,subject"
	if err := validateOutputFlags(); err != nil || outputFormat() != output.FormatQuiet {
		t.Fatalf("results-only with select = %v, format %v", err, outputFormat())
	}

	idsOnlyFlag = true
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("results-only with ids-only = %v", err)
	}

	resultsOnlyFlag, idsOnlyFlag, countFlag = false, false, true
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("select with count = %v", err)
	}

	countFlag, selectFlag = false, "   , "
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("empty select = %v", err)
	}

	quietFlag, resultsOnlyFlag, selectFlag = true, true, ""
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("quiet with results-only = %v", err)
	}
}

func TestPrescanAutomationOutputFlagsStopsAtCommand(t *testing.T) {
	previousResultsOnly, previousSelect, previousFailEmpty := resultsOnlyFlag, selectFlag, failEmptyFlag
	resultsOnlyFlag, selectFlag, failEmptyFlag = false, "", false
	t.Cleanup(func() {
		resultsOnlyFlag, selectFlag, failEmptyFlag = previousResultsOnly, previousSelect, previousFailEmpty
	})

	prescanAutomationFlags([]string{"--results-only", "--select=id,subject", "--fail-empty", "mail", "--select=ignored"})
	if !resultsOnlyFlag || selectFlag != "id,subject" || !failEmptyFlag {
		t.Fatalf("prescanned flags = results-only:%v select:%q fail-empty:%v", resultsOnlyFlag, selectFlag, failEmptyFlag)
	}
}

func TestCollectionPageUsesOneItemLookahead(t *testing.T) {
	values, option := collectionPage([]string{"one", "two", "three"}, 2)
	if !reflect.DeepEqual(values, []string{"one", "two"}) {
		t.Fatalf("page = %#v", values)
	}
	response := output.Response{}
	option(&response)
	if response.Meta["has_more"] != true {
		t.Fatalf("page metadata = %#v", response.Meta)
	}
}
