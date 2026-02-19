package models

// Config represents the application configuration
type Config struct {
	API           APIConfig           `yaml:"api"`
	Database      DatabaseConfig      `yaml:"database"`
	Qdrant        QdrantConfig        `yaml:"qdrant"`
	Redis         RedisConfig         `yaml:"redis"`
	Embedding     EmbeddingConfig     `yaml:"embedding"`
	Decay         DecayConfig         `yaml:"decay"`
	Consolidation ConsolidationConfig `yaml:"consolidation"`
	Criticality   CriticalityConfig   `yaml:"criticality"`
	Retention     RetentionConfig     `yaml:"retention"`
}

type APIConfig struct {
	Port                  int    `yaml:"port"`
	LogLevel              string `yaml:"log_level"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
}

type DatabaseConfig struct {
	Host                     string `yaml:"host"`
	Port                     int    `yaml:"port"`
	Name                     string `yaml:"name"`
	User                     string `yaml:"user"`
	Password                 string `yaml:"password"`
	MaxConnections           int    `yaml:"max_connections"`
	ConnectionTimeoutSeconds int    `yaml:"connection_timeout_seconds"`
}

type QdrantConfig struct {
	URL        string `yaml:"url"`
	Collection string `yaml:"collection"`
	VectorSize int    `yaml:"vector_size"`
	Distance   string `yaml:"distance"`
}

type RedisConfig struct {
	URL            string `yaml:"url"`
	TTLHours       int    `yaml:"ttl_hours"`
	MaxConnections int    `yaml:"max_connections"`
}

type EmbeddingConfig struct {
	Provider       string `yaml:"provider"`
	Endpoint       string `yaml:"endpoint"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type DecayConfig struct {
	ThresholdDays  int     `yaml:"threshold_days"`
	DecayRate      float64 `yaml:"decay_rate"`
	MinCriticality float64 `yaml:"min_criticality"`
}

type ConsolidationConfig struct {
	IntervalHours       int     `yaml:"interval_hours"`
	BatchSize           int     `yaml:"batch_size"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
}

type CriticalityConfig struct {
	OperatorCorrectionWeight float64 `yaml:"operator_correction_weight"`
	FailureWeight            float64 `yaml:"failure_weight"`
	SuccessWeight            float64 `yaml:"success_weight"`
	DecayWeight              float64 `yaml:"decay_weight"`
	ReferencedWeight         float64 `yaml:"referenced_weight"`
}

type RetentionConfig struct {
	MinDays          int     `yaml:"min_days"`
	ArchiveThreshold float64 `yaml:"archive_threshold"`
	DeleteThreshold  float64 `yaml:"delete_threshold"`
}
