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
	previousCount, previousSelect, previousSelectProvided, previousFailEmpty := countFlag, selectFlag, selectProvidedFlag, failEmptyFlag
	t.Cleanup(func() {
		jsonFlag, quietFlag = previousJSON, previousQuiet
		resultsOnlyFlag, idsOnlyFlag = previousResultsOnly, previousIDsOnly
		countFlag, selectFlag, selectProvidedFlag, failEmptyFlag = previousCount, previousSelect, previousSelectProvided, previousFailEmpty
	})

	jsonFlag, quietFlag, resultsOnlyFlag, idsOnlyFlag, countFlag = false, false, true, false, false
	selectFlag = "id,subject"
	if err := validateOutputFlags(); err != nil || outputFormat() != output.FormatResultsOnly {
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

	quietFlag, resultsOnlyFlag, selectFlag = false, false, "id"
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("select without structured output = %v", err)
	}

	selectFlag, failEmptyFlag = "", true
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("fail-empty without structured output = %v", err)
	}

	jsonFlag, failEmptyFlag, selectProvidedFlag = true, false, true
	if err := validateOutputFlags(); apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("explicit empty select = %v", err)
	}
}

func TestPrescanAutomationOutputFlagsStopsAtCommand(t *testing.T) {
	previousResultsOnly, previousSelect, previousFailEmpty := resultsOnlyFlag, selectFlag, failEmptyFlag
	previousSelectProvided := selectProvidedFlag
	resultsOnlyFlag, selectFlag, failEmptyFlag, selectProvidedFlag = false, "", false, false
	t.Cleanup(func() {
		resultsOnlyFlag, selectFlag, failEmptyFlag = previousResultsOnly, previousSelect, previousFailEmpty
		selectProvidedFlag = previousSelectProvided
	})

	prescanAutomationFlags([]string{"--results-only", "--select=id,subject", "--fail-empty", "mail", "--select=ignored"})
	if !resultsOnlyFlag || selectFlag != "id,subject" || !selectProvidedFlag || !failEmptyFlag {
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

	values, option = collectionPage([]string{"one", "two"}, 2)
	response = output.Response{}
	option(&response)
	if response.Meta["has_more"] != false || response.Meta["total"] != 2 {
		t.Fatalf("exhausted page metadata = %#v", response.Meta)
	}
}

func TestProjectionIsRejectedBeforeMicrosoft365Mutation(t *testing.T) {
	previousJSON, previousSelect, previousDryRun := jsonFlag, selectFlag, dryRunFlag
	previousWriter := writer
	jsonFlag, selectFlag, dryRunFlag = true, "invented", false
	t.Cleanup(func() {
		jsonFlag, selectFlag, dryRunFlag = previousJSON, previousSelect, previousDryRun
		writer = previousWriter
	})

	err := prepareCommand(mailSendCmd, nil)
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("projection on mail send = %v", err)
	}
}

func TestUnsupportedCommandOutputModeIsRejectedInPreflight(t *testing.T) {
	previousResultsOnly, previousSelectProvided := resultsOnlyFlag, selectProvidedFlag
	previousWriter := writer
	resultsOnlyFlag, selectProvidedFlag = true, false
	t.Cleanup(func() {
		resultsOnlyFlag, selectProvidedFlag = previousResultsOnly, previousSelectProvided
		writer = previousWriter
	})

	err := prepareCommand(skillCmd, nil)
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("results-only on human-only skill command = %v", err)
	}
}
