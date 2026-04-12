package domain

const (
	GenAISystemOpenAI     = "openai"
	GenAISystemAnthropic  = "anthropic"
	GenAISystemVertexAI   = "vertex_ai"
	GenAISystemBedrock    = "bedrock"
	GenAISystemOllama     = "ollama"
	GenAISystemAzure      = "azure"
	GenAISystemGCP        = "gcp"
	GenAISystemCustom     = "custom"
)

const (
	GenAIOperationChat       = "chat"
	GenAIOperationCompletion = "completion"
	GenAIOperationEmbeddings = "embeddings"
	GenAIOperationFineTune   = "fine_tune"
)

const (
	GenAIToolTypeFunction        = "function"
	GenAIToolTypeCodeInterpreter = "code_interpreter"
	GenAIToolTypeRetrieval       = "retrieval"
	GenAIToolTypeImageGeneration = "image_generation"
)

type OTelAttribute string

const (
	AttrGenAISystem                   OTelAttribute = "gen_ai.system"
	AttrGenAIOperationName            OTelAttribute = "gen_ai.operation.name"
	AttrGenAIRequestModel             OTelAttribute = "gen_ai.request.model"
	AttrGenAIRequestMaxTokens         OTelAttribute = "gen_ai.request.max_tokens"
	AttrGenAIRequestTemperature       OTelAttribute = "gen_ai.request.temperature"
	AttrGenAIRequestTopP              OTelAttribute = "gen_ai.request.top_p"
	AttrGenAIRequestPresencePenalty   OTelAttribute = "gen_ai.request.presence_penalty"
	AttrGenAIRequestFrequencyPenalty  OTelAttribute = "gen_ai.request.frequency_penalty"
	AttrGenAIRequestStopSequences     OTelAttribute = "gen_ai.request.stop_sequences"
	AttrGenAIResponseModel            OTelAttribute = "gen_ai.response.model"
	AttrGenAIResponseFinishReasons    OTelAttribute = "gen_ai.response.finish_reasons"
	AttrGenAIResponseID               OTelAttribute = "gen_ai.response.id"
	AttrGenAIUsageInputTokens         OTelAttribute = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutputTokens        OTelAttribute = "gen_ai.usage.output_tokens"
	AttrGenAIUsageTotalTokens         OTelAttribute = "gen_ai.usage.total_tokens"
	AttrGenAIToolName                 OTelAttribute = "gen_ai.tool.name"
	AttrGenAIToolType                 OTelAttribute = "gen_ai.tool.type"
	AttrGenAIToolDescription          OTelAttribute = "gen_ai.tool.description"
	AttrGenAIToolCallID               OTelAttribute = "gen_ai.tool.call.id"
	AttrGenAIToolCallArguments        OTelAttribute = "gen_ai.tool.call.arguments"
	AttrGenAIToolResult               OTelAttribute = "gen_ai.tool.result"
)

type OTelMetric string

const (
	MetricGenAIClientTokenUsage       OTelMetric = "gen_ai.client.token.usage"
	MetricGenAIClientOperationDuration OTelMetric = "gen_ai.client.operation.duration"
	MetricGenAIClientRequestDuration   OTelMetric = "gen_ai.client.request.duration"
)

type DCGMMetric string

const (
	DCGMFieldGPUUtil        DCGMMetric = "DCGM_FI_DEV_GPU_UTIL"
	DCGMFieldMemCopyUtil    DCGMMetric = "DCGM_FI_DEV_MEM_COPY_UTIL"
	DCGMFieldFBUsed         DCGMMetric = "DCGM_FI_DEV_FB_USED"
	DCGMFieldFBFree         DCGMMetric = "DCGM_FI_DEV_FB_FREE"
	DCGMFieldPowerUsage     DCGMMetric = "DCGM_FI_DEV_POWER_USAGE"
	DCGMFieldPowerDraw      DCGMMetric = "DCGM_FI_DEV_POWER_DRAW"
	DCGMFieldGPUTemp        DCGMMetric = "DCGM_FI_DEV_GPU_TEMP"
	DCGMFieldSMClock        DCGMMetric = "DCGM_FI_DEV_SM_CLOCK"
	DCGMFieldMemClock       DCGMMetric = "DCGM_FI_DEV_MEM_CLOCK"
	DCGMFieldPCIeRxThroughput DCGMMetric = "DCGM_FI_DEV_PCIE_RX_THROUGHPUT"
	DCGMFieldPCIeTxThroughput DCGMMetric = "DCGM_FI_DEV_PCIE_TX_THROUGHPUT"
	DCGMFieldXIDErrors      DCGMMetric = "DCGM_FI_DEV_XID_ERRORS"
)

type vLLMMetric string

const (
	VLLMMetricNumRequestsRunning   vLLMMetric = "vllm:num_requests_running"
	VLLMMetricNumRequestsWaiting   vLLMMetric = "vllm:num_requests_waiting"
	VLLMMetricGPUCacheUsagePerc    vLLMMetric = "vllm:gpu_cache_usage_perc"
	VLLMMetricTimeToFirstToken     vLLMMetric = "vllm:time_to_first_token_seconds"
	VLLMMetricTimePerOutputToken   vLLMMetric = "vllm:time_per_output_token_seconds"
	VLLMMetricE2ERequestLatency    vLLMMetric = "vllm:e2e_request_latency_seconds"
)

type InferenceEngine string

const (
	InferenceEngineVLLM      InferenceEngine = "vllm"
	InferenceEngineTriton    InferenceEngine = "triton"
	InferenceEngineTensorRT  InferenceEngine = "tensorrt"
	InferenceEngineTGI       InferenceEngine = "tgi"
	InferenceEngineOpenAI    InferenceEngine = "openai-compatible"
)

type CostType string

const (
	CostTypeToken   CostType = "token"
	CostTypeCompute CostType = "compute"
	CostTypeStorage CostType = "storage"
	CostTypeNetwork CostType = "network"
)

type BudgetScopeType string

const (
	BudgetScopeGlobal  BudgetScopeType = "global"
	BudgetScopeService BudgetScopeType = "service"
	BudgetScopeUser    BudgetScopeType = "user"
	BudgetScopeTeam    BudgetScopeType = "team"
	BudgetScopeModel   BudgetScopeType = "model"
)

type BudgetPeriod string

const (
	BudgetPeriodDaily    BudgetPeriod = "daily"
	BudgetPeriodWeekly   BudgetPeriod = "weekly"
	BudgetPeriodMonthly  BudgetPeriod = "monthly"
	BudgetPeriodQuarterly BudgetPeriod = "quarterly"
)

type BudgetAction string

const (
	BudgetActionThrottle  BudgetAction = "throttle"
	BudgetActionDowngrade BudgetAction = "downgrade"
	BudgetActionBlock     BudgetAction = "block"
	BudgetActionNotify    BudgetAction = "notify"
)

type JobType string

const (
	JobTypeTraining   JobType = "training"
	JobTypeInference  JobType = "inference"
	JobTypeBatch      JobType = "batch"
	JobTypeEvaluation JobType = "evaluation"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

type AlertType string

const (
	AlertTypeXIDError    AlertType = "xid_error"
	AlertTypeTemperature AlertType = "temperature"
	AlertTypeMemory      AlertType = "memory"
	AlertTypePower       AlertType = "power"
	AlertTypeECC         AlertType = "ecc"
)

type DataClassification string

const (
	DataClassificationPublic      DataClassification = "public"
	DataClassificationInternal    DataClassification = "internal"
	DataClassificationConfidential DataClassification = "confidential"
	DataClassificationRestricted  DataClassification = "restricted"
)

type ActionType string

const (
	ActionAllowed    ActionType = "allowed"
	ActionBlocked    ActionType = "blocked"
	ActionSanitized  ActionType = "sanitized"
	ActionFlagged    ActionType = "flagged"
)
