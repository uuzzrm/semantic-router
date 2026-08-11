package recipe

import (
	"encoding/json"
	"testing"
)

func TestCompareEvalResponseNormalizesDefaultRecipeForTrace(t *testing.T) {
	probes := []ProbeDetail{
		{ProbeSummary: ProbeSummary{Expected: ExpectedAssertions{Decision: "decision-a"}}},
		{ProbeSummary: ProbeSummary{Expected: ExpectedAssertions{Decision: "decision-b", Recipe: "default"}}},
	}
	response := json.RawMessage(`{
  "recipe":"default",
  "routing_decision":"decision-a",
  "decision_result":{"decision_name":"decision-a"},
  "eval_trace":[
    {"decision_name":"decision-a","matched":true},
    {"decision_name":"decision-b","matched":false}
  ]
}`)

	_, checks, failures, err := compareEvalResponse(response, probes[0], probes)
	if err != nil {
		t.Fatalf("compareEvalResponse(): %v", err)
	}
	if !checks.Recipe || !checks.Trace || len(failures) != 0 {
		t.Fatalf("checks = %#v, failures = %v", checks, failures)
	}
}

func TestAllowedDecisionsNormalizesRecipeNames(t *testing.T) {
	probes := []ProbeDetail{
		{ProbeSummary: ProbeSummary{Expected: ExpectedAssertions{Decision: "implicit", Recipe: ""}}},
		{ProbeSummary: ProbeSummary{Expected: ExpectedAssertions{Decision: "explicit", Recipe: " default "}}},
		{ProbeSummary: ProbeSummary{Expected: ExpectedAssertions{Decision: "named", Recipe: "balanced"}}},
	}

	allowed := allowedDecisions(probes, "default")
	if _, found := allowed["implicit"]; !found {
		t.Fatal("implicit default decision is missing")
	}
	if _, found := allowed["explicit"]; !found {
		t.Fatal("explicit default decision is missing")
	}
	if _, found := allowed["named"]; found {
		t.Fatal("named recipe decision must not be included")
	}
	if len(allowed) != 2 {
		t.Fatalf("allowed decisions = %v, want exactly the two default decisions", allowed)
	}

	_, passed, failures := compareTrace(
		[]evalTrace{{DecisionName: "implicit", Matched: true}},
		"implicit",
		allowed,
	)
	if passed || len(failures) == 0 {
		t.Fatal("trace without the sibling default decision must fail strict validation")
	}

	_, passed, failures = compareTrace(
		[]evalTrace{
			{DecisionName: "implicit", Matched: true},
			{DecisionName: "explicit", Matched: false},
			{DecisionName: "named", Matched: false},
		},
		"implicit",
		allowed,
	)
	if passed || len(failures) == 0 {
		t.Fatal("trace containing a named-recipe decision must fail strict validation")
	}
}
