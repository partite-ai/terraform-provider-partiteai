package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Model Complexity constants.
const (
	modelComplexityLow    = "LOW"
	modelComplexityMedium = "MEDIUM"
	modelComplexityHigh   = "HIGH"
)

// Memory Scope constants.
const (
	memoryScopeGlobal  = "GLOBAL"
	memoryScopeLocal   = "LOCAL"
	memoryScopeRequest = "REQUEST"
)

// commonValidators provides reusable validators.
var commonValidators = struct {
	modelComplexity validator.String
	memoryScope     validator.String
}{
	modelComplexity: stringvalidator.OneOf(
		modelComplexityLow,
		modelComplexityMedium,
		modelComplexityHigh,
	),
	memoryScope: stringvalidator.OneOf(
		memoryScopeGlobal,
		memoryScopeLocal,
		memoryScopeRequest,
	),
}
