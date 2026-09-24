package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Evaluation configuration exists only on an enterprise deployment, so these
// run against that backend alone. TestAccEval_needsEnterprise covers what
// happens anywhere else.

const userMetricHCL = `
resource "partiteai_user_metric" "test" {
  name        = "acc-user-metric"
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

func TestAccUserMetric_basic(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: userMetricHCL + `
data "partiteai_user_metric" "test" {
  id = partiteai_user_metric.test.id
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "name", "acc-user-metric"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "complexity", "LOW"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "examples.#", "2"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "examples.0.rating", "1"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "input_fields.#", "1"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "input_fields.0.type", "STRING"),
				resource.TestCheckResourceAttrPair(
					"data.partiteai_user_metric.test", "prompt",
					"partiteai_user_metric.test", "prompt"),
			),
		},
		// The two lists are replaced wholesale, so dropping an example must
		// leave one behind rather than merging.
		resource.TestStep{
			Config: `
resource "partiteai_user_metric" "test" {
  name        = "acc-user-metric-renamed"
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
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "name", "acc-user-metric-renamed"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "complexity", "MEDIUM"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "examples.#", "1"),
				resource.TestCheckResourceAttr("partiteai_user_metric.test", "examples.0.rating", "5"),
			),
		},
		// Dropping the description has to remove it. The API leaves alone what
		// it is not told about, so this only works because the provider sends
		// the field as an explicit null.
		resource.TestStep{
			Config: `
resource "partiteai_user_metric" "test" {
  name       = "acc-user-metric-renamed"
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
			Check: resource.TestCheckNoResourceAttr("partiteai_user_metric.test", "description"),
		},
		resource.TestStep{
			Config:            userMetricHCL,
			ResourceName:      "partiteai_user_metric.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	)
}

// TestAccSystemMetric_dataSource reads two metrics the product ships: one a
// model judges, one worked out from the trace.
func TestAccSystemMetric_dataSource(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_system_metric" "judged" {
  key = "instruction_following"
}

data "partiteai_system_metric" "computed" {
  key = "latency"
}
`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.partiteai_system_metric.judged", "type", "JUDGED"),
			resource.TestCheckResourceAttrSet("data.partiteai_system_metric.judged", "name"),
			resource.TestCheckResourceAttrSet("data.partiteai_system_metric.judged", "prompt"),
			resource.TestCheckResourceAttr("data.partiteai_system_metric.computed", "type", "COMPUTED"),
			resource.TestCheckResourceAttrSet("data.partiteai_system_metric.computed", "name"),
		),
	})
}

// TestAccSystemMetric_missingKey pins what a key that matches nothing does:
// the field is non-null, so the API says it found none and the provider passes
// that through.
func TestAccSystemMetric_missingKey(t *testing.T) {
	accTestEnterprise(t, resource.TestStep{
		Config: `
data "partiteai_system_metric" "missing" {
  key = "no_such_metric"
}
`,
		ExpectError: regexp.MustCompile(`(?s)No system metric has the key`),
	})
}

// rubricHCL scores against one metric of its own and one that came with the
// product, which is the pair of branches the union mapping has to get right.
const rubricHCL = userMetricHCL + `
resource "partiteai_rubric" "test" {
  name        = "acc-rubric"
  description = "How well the assistant answered."

  metrics = [
    {
      user_metric_id = partiteai_user_metric.test.id
      trial_count    = 2
      weight         = 1
      input_values   = jsonencode({ required_source = "the handbook" })
    },
    {
      system_metric_key = "latency"
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
					"system_metric_key": "latency",
					"lower_is_better":   "true",
					"score_ceiling":     "5000",
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
			Config: userMetricHCL + `
resource "partiteai_rubric" "test" {
  name        = "acc-rubric"
  description = "How well the assistant answered."

  metrics = [
    {
      user_metric_id = partiteai_user_metric.test.id
      trial_count    = 3
      weight         = 2
      input_values   = jsonencode({ required_source = "the handbook" })
    },
    {
      # A weighted computed metric has to say what range it scores over.
      system_metric_key = "llm_call_count"
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
					"system_metric_key": "llm_call_count",
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

// TestAccRubric_metricNamesOneSource pins that a rubric entry names a metric of
// yours or one of the product's, never both and never neither.
func TestAccRubric_metricNamesOneSource(t *testing.T) {
	accTestEnterprise(t,
		resource.TestStep{
			Config: userMetricHCL + `
resource "partiteai_rubric" "test" {
  name = "acc-rubric-both"
  metrics = [
    {
      user_metric_id    = partiteai_user_metric.test.id
      system_metric_key = "latency"
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
		resource.TestStep{Config: userMetricHCL},
	)
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
