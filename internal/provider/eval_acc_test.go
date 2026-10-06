package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Evaluation configuration exists only on an enterprise deployment, so these
// run against that backend alone. TestAccEval_needsEnterprise covers what
// happens anywhere else.

const judgeMetricHCL = `
resource "partiteai_judge_metric" "test" {
  name        = "acc-judge-metric"
  category    = "QUALITY"
  scope       = "HOP"
  description = "Whether the answer cited a source."
  prompt      = "Did the assistant cite a source for its answer?"
  complexity  = "LOW"

  examples = [
    {
      rating   = 1
      scenario = "The assistant answered with no source at all."
      judgment = "Nothing to check the answer against."
    },
    {
      rating   = 5
      scenario = "The assistant quoted the document it drew on."
      judgment = "The reader can follow the claim back."
    },
  ]

  input_fields = [
    {
      name        = "required_source"
      label       = "Required source"
      description = "Source the answer is expected to cite."
      type        = "STRING"
      required    = true
    },
  ]
}
`

func TestAccJudgeMetric_basic(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: judgeMetricHCL + `
data "partiteai_judge_metric" "test" {
  id = partiteai_judge_metric.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "name", "acc-judge-metric"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "complexity", "LOW"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "examples.#", "2"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "examples.0.rating", "1"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "input_fields.#", "1"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "input_fields.0.type", "STRING"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_judge_metric.test", "prompt",
					"partiteai_judge_metric.test", "prompt"),
			),
		},
		// The two lists are replaced wholesale, so dropping an example must
		// leave one behind rather than merging.
		resource.TestStep{
			Config: `
resource "partiteai_judge_metric" "test" {
  name        = "acc-judge-metric-renamed"
  category    = "QUALITY"
  scope       = "HOP"
  description = "Whether the answer cited a source."
  prompt      = "Did the assistant cite a source it was given?"
  complexity  = "MEDIUM"

  examples = [
    {
      rating   = 5
      scenario = "The assistant quoted the document it drew on."
      judgment = "The reader can follow the claim back."
    },
  ]

  input_fields = [
    {
      name        = "required_source"
      label       = "Required source"
      description = "Source the answer is expected to cite."
      type        = "STRING"
      required    = true
    },
  ]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "name", "acc-judge-metric-renamed"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "complexity", "MEDIUM"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "examples.#", "1"),
				resource.TestCheckResourceAttr("partiteai_judge_metric.test", "examples.0.rating", "5"),
			),
		},
		// Dropping the description has to remove it. The API leaves alone what
		// it is not told about, so this only works because the provider sends
		// the field as an explicit null.
		resource.TestStep{
			Config: `
resource "partiteai_judge_metric" "test" {
  name       = "acc-judge-metric-renamed"
  category    = "QUALITY"
  scope       = "HOP"
  prompt     = "Did the assistant cite a source it was given?"
  complexity = "MEDIUM"

  examples = [
    {
      rating   = 5
      scenario = "The assistant quoted the document it drew on."
      judgment = "The reader can follow the claim back."
    },
  ]

  input_fields = [
    {
      name        = "required_source"
      label       = "Required source"
      description = "Source the answer is expected to cite."
      type        = "STRING"
      required    = true
    },
  ]
}
`,
			Check: resource.TestCheckNoResourceAttr("partiteai_judge_metric.test", "description"),
		},
		resource.TestStep{
			Config:            judgeMetricHCL,
			ResourceName:      "partiteai_judge_metric.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccJudgeMetric_findsABuiltInByName reads one of the judge metrics every
// organization starts with, by its name.
func TestAccJudgeMetric_findsABuiltInByName(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_judge_metric" "instruction_following" {
  name = "Instruction Following"
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.partiteai_judge_metric.instruction_following", "id"),
			resource.TestCheckResourceAttr("data.partiteai_judge_metric.instruction_following", "category", "QUALITY"),
			resource.TestCheckResourceAttr("data.partiteai_judge_metric.instruction_following", "scope", "HOP"),
			resource.TestCheckResourceAttrSet("data.partiteai_judge_metric.instruction_following", "prompt"),
		),
	})
}

// TestAccJudgeMetric_missingName pins that a name only part of another metric's
// name matches nothing: the search finds by text, and the data source keeps an
// exact match only.
func TestAccJudgeMetric_missingName(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_judge_metric" "partial" {
  name = "Instruction"
}
`,
		ExpectError: regexp.MustCompile(`(?s)no judge metric named "Instruction"`),
	})
}

func TestAccComputedMetric_dataSource(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_computed_metric" "latency" {
  key = "latency"
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.partiteai_computed_metric.latency", "name"),
			resource.TestCheckResourceAttrSet("data.partiteai_computed_metric.latency", "unit"),
			resource.TestCheckResourceAttrSet("data.partiteai_computed_metric.latency", "aggregation"),
		),
	})
}

// TestAccComputedMetric_missingKey pins what a key that matches nothing does:
// the API says it found none and the provider passes that through.
func TestAccComputedMetric_missingKey(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_computed_metric" "missing" {
  key = "no_such_metric"
}
`,
		ExpectError: regexp.MustCompile(`(?s)No computed metric has the key`),
	})
}

// rubricHCL scores against one judge metric and one computed metric, which is
// the pair of branches the union mapping has to get right.
const rubricHCL = judgeMetricHCL + `
resource "partiteai_rubric" "test" {
  name        = "acc-rubric"
  description = "How well the assistant answered."

  metrics = [
    {
      judge_metric_id = partiteai_judge_metric.test.id
      trial_count    = 2
      weight         = 1
      input_values   = jsonencode({ required_source = "the handbook" })
    },
    {
      computed_metric_key = "latency"
      score_floor       = 0
      score_ceiling     = 5000
      lower_is_better   = true
      weight            = 0.5
    },
  ]
}
`

func TestAccRubric_basic(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: rubricHCL + `
data "partiteai_rubric" "test" {
  id = partiteai_rubric.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_rubric.test", "name", "acc-rubric"),
				resource.TestCheckResourceAttr("partiteai_rubric.test", "metrics.#", "2"),
				// The metrics are a set, so each is checked by value rather
				// than by where it sits.
				resource.TestCheckTypeSetElemNestedAttrs("partiteai_rubric.test", "metrics.*", map[string]string{
					"computed_metric_key": "latency",
					"lower_is_better":     "true",
					"score_ceiling":       "5000",
					// A computed metric takes no input values, so the API does
					// not report any for one.
					"input_values": "",
				}),
				resource.TestCheckTypeSetElemNestedAttrs("partiteai_rubric.test", "metrics.*", map[string]string{
					"trial_count":  "2",
					"input_values": `{"required_source":"the handbook"}`,
				}),
				resource.TestCheckResourceAttr("data.partiteai_rubric.test", "metrics.#", "2"),
			),
		},
		// An update is a difference rather than a whole list: this changes one
		// metric's settings, drops the other and adds a third.
		resource.TestStep{
			Config: judgeMetricHCL + `
resource "partiteai_rubric" "test" {
  name        = "acc-rubric"
  description = "How well the assistant answered."

  metrics = [
    {
      judge_metric_id = partiteai_judge_metric.test.id
      trial_count    = 3
      weight         = 2
      input_values   = jsonencode({ required_source = "the handbook" })
    },
    {
      # A weighted computed metric has to say what range it scores over.
      computed_metric_key = "llm_call_count"
      score_floor       = 0
      score_ceiling     = 50
      lower_is_better   = true
      weight            = 0.25
    },
  ]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_rubric.test", "metrics.#", "2"),
				resource.TestCheckTypeSetElemNestedAttrs("partiteai_rubric.test", "metrics.*", map[string]string{
					"trial_count": "3",
					"weight":      "2",
				}),
				resource.TestCheckTypeSetElemNestedAttrs("partiteai_rubric.test", "metrics.*", map[string]string{
					"computed_metric_key": "llm_call_count",
				}),
			),
		},
		resource.TestStep{
			Config:            rubricHCL,
			ResourceName:      "partiteai_rubric.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccRubric_metricNamesOneSource pins that a rubric entry names a judge
// metric or a computed one, never both and never neither.
func TestAccRubric_metricNamesOneSource(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: judgeMetricHCL + `
resource "partiteai_rubric" "test" {
  name = "acc-rubric-both"
  metrics = [
    {
      judge_metric_id    = partiteai_judge_metric.test.id
      computed_metric_key = "latency"
    },
  ]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
		resource.TestStep{
			Config: `
resource "partiteai_rubric" "test" {
  name    = "acc-rubric-neither"
  metrics = [{ weight = 1 }]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
		resource.TestStep{Config: judgeMetricHCL},
	)
}

// A rubric entry takes the settings of its kind of metric only, and the
// provider refuses the others before anything is sent.
func TestAccRubric_settingsFitTheKindOfMetric(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: `
resource "partiteai_rubric" "test" {
  name    = "acc-rubric-computed-trials"
  metrics = [{ computed_metric_key = "latency", trial_count = 2 }]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
		resource.TestStep{
			Config: judgeMetricHCL + `
resource "partiteai_rubric" "test" {
  name    = "acc-rubric-empty-inputs"
  metrics = [{ judge_metric_id = partiteai_judge_metric.test.id, input_values = jsonencode({}) }]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Empty input values`),
		},
		resource.TestStep{
			Config: judgeMetricHCL + `
resource "partiteai_rubric" "test" {
  name    = "acc-rubric-judge-bounds"
  metrics = [{ judge_metric_id = partiteai_judge_metric.test.id, score_floor = 0 }]
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
		},
		resource.TestStep{Config: judgeMetricHCL},
	)
}

// A rubric can score against a built-in judge metric found by name.
func TestAccRubric_withABuiltInJudge(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_judge_metric" "instruction_following" {
  name = "Instruction Following"
}

resource "partiteai_rubric" "test" {
  name = "acc-rubric-built-in"
  metrics = [
    { judge_metric_id = data.partiteai_judge_metric.instruction_following.id },
  ]
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("partiteai_rubric.test", "metrics.#", "1"),
			resource.TestCheckResourceAttrPair(
				"partiteai_rubric.test", "metrics.0.judge_metric_id",
				"data.partiteai_judge_metric.instruction_following", "id"),
			resource.TestCheckResourceAttr("partiteai_rubric.test", "metrics.0.trial_count", "1"),
		),
	})
}

const evalMeshMonitorHCL = rubricHCL + baseMesh + `
resource "partiteai_eval_mesh_monitor" "test" {
  mesh_id      = partiteai_mesh.base.id
  rubric_id    = partiteai_rubric.test.id
  model_set_id = partiteai_model_set.base.id
  enabled      = true
  sample_rate  = 0.25
}
`

func TestAccEvalMeshMonitor_basic(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: evalMeshMonitorHCL + `
data "partiteai_eval_mesh_monitor" "test" {
  id = partiteai_eval_mesh_monitor.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_eval_mesh_monitor.test", "enabled", "true"),
				resource.TestCheckResourceAttr("partiteai_eval_mesh_monitor.test", "sample_rate", "0.25"),
				resource.TestCheckResourceAttrPair(
					"partiteai_eval_mesh_monitor.test", "mesh_id",
					"partiteai_mesh.base", "id"),
				resource.TestCheckResourceAttrPair(
					"partiteai_eval_mesh_monitor.test", "rubric_id",
					"partiteai_rubric.test", "id"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_eval_mesh_monitor.test", "sample_rate",
					"partiteai_eval_mesh_monitor.test", "sample_rate"),
			),
		},
		// Turning one off and changing its share are updates, not replacements.
		resource.TestStep{
			Config: rubricHCL + baseMesh + `
resource "partiteai_eval_mesh_monitor" "test" {
  mesh_id      = partiteai_mesh.base.id
  rubric_id    = partiteai_rubric.test.id
  model_set_id = partiteai_model_set.base.id
  enabled      = false
  sample_rate  = 1
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_eval_mesh_monitor.test", "enabled", "false"),
				resource.TestCheckResourceAttr("partiteai_eval_mesh_monitor.test", "sample_rate", "1"),
			),
		},
		resource.TestStep{
			Config:            evalMeshMonitorHCL,
			ResourceName:      "partiteai_eval_mesh_monitor.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccEvalMeshMonitor_sampleRateRange pins the bounds the API expects.
func TestAccEvalMeshMonitor_sampleRateRange(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: rubricHCL + baseMesh + `
resource "partiteai_eval_mesh_monitor" "test" {
  mesh_id      = partiteai_mesh.base.id
  rubric_id    = partiteai_rubric.test.id
  model_set_id = partiteai_model_set.base.id
  enabled      = true
  sample_rate  = 1.5
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value`),
		},
		resource.TestStep{Config: rubricHCL + baseMesh},
	)
}
