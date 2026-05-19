package model

import "time"

type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "hml"
	EnvProd    Environment = "prd"
)

type Criticality string

const (
	CriticalityLow      Criticality = "low"
	CriticalityMedium   Criticality = "medium"
	CriticalityHigh     Criticality = "high"
	CriticalityCritical Criticality = "critical"
)

type WorkloadContext struct {
	TenantID         int64       `json:"tenant_id"`
	WorkloadUID      string      `json:"workload_uid"`
	DeploymentName   string      `json:"deployment_name"`
	Namespace        string      `json:"namespace"`
	Cluster          string      `json:"cluster"`
	Environment      Environment `json:"environment"`
	Criticality      Criticality `json:"criticality"`
	HasDatadog       bool        `json:"has_datadog"`
	ServiceCatalogID string      `json:"service_catalog_id,omitempty"`
}

type HpaCurrent struct {
	MinReplicas    int `json:"min_replicas"`
	MaxReplicas    int `json:"max_replicas"`
	TargetCPUPct   int `json:"target_cpu_pct"`
	TargetMemPct   int `json:"target_mem_pct,omitempty"`
	CurrentReplica int `json:"current_replicas,omitempty"`
}

type RecommendationSource string

const (
	SourceDatadogP95 RecommendationSource = "datadog_p95_30d"
	SourceTemplate   RecommendationSource = "template"
	SourceSkipped    RecommendationSource = "skipped"
)

type HpaRecommendation struct {
	WorkloadUID  string               `json:"workload_uid"`
	MinReplicas  int                  `json:"min_replicas"`
	MaxReplicas  int                  `json:"max_replicas"`
	TargetCPUPct int                  `json:"target_cpu_pct"`
	TargetMemPct int                  `json:"target_mem_pct,omitempty"`
	Source       RecommendationSource `json:"source"`
	Confidence   float64              `json:"confidence"`
	WindowDays   int                  `json:"window_days,omitempty"`
	ComputedAt   time.Time            `json:"computed_at"`
	Notes        string               `json:"notes,omitempty"`
}

type HpaTemplate struct {
	ID           int64       `json:"id"`
	TenantID     int64       `json:"tenant_id"`
	Environment  Environment `json:"environment"`
	Criticality  Criticality `json:"criticality"`
	MinReplicas  int         `json:"min_replicas"`
	MaxReplicas  int         `json:"max_replicas"`
	TargetCPUPct int         `json:"target_cpu_pct"`
	TargetMemPct int         `json:"target_mem_pct,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type UsageSnapshot struct {
	WindowDays         int     `json:"window_days"`
	CPUP95Pct          float64 `json:"cpu_p95_pct"`
	CPUP99Pct          float64 `json:"cpu_p99_pct"`
	ReplicasPeak       int     `json:"replicas_peak"`
	ReplicasAverage    float64 `json:"replicas_avg"`
	SamplesCount       int     `json:"samples_count"`
	ConfidenceFraction float64 `json:"confidence"`
}
