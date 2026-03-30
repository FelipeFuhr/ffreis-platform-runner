package executor

const (
	terraformBinary = "terraform"

	terraformSubcommandPlan  = "plan"
	terraformSubcommandApply = "apply"

	terraformPlanFile = "plan.tfplan"

	errApplyConfirmRequired = "apply requires --confirm flag"
)
