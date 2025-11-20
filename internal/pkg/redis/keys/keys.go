package keys

import "fmt"

// Namespaces/prefixes
const (
	PrefixDelivered = "delivered"
	PrefixMetrics   = "metrics"
	PrefixDelayed   = "zset:notifications:delayed"
)

// DeliveredKey returns key marking a notification id as delivered (idempotency)
// Example: delivered:<notificationID>
func DeliveredKey(notificationID string) string {
	return fmt.Sprintf("%s:%s", PrefixDelivered, notificationID)
}

// MetricsDailyKey returns a metrics key for a metric name and YYYYMMDD date
// Example: metrics:notifications:sent:20250131
func MetricsDailyKey(metric, yyyymmdd string) string {
	return fmt.Sprintf("%s:%s:%s", PrefixMetrics, metric, yyyymmdd)
}

// DelayedZSetKey returns the canonical delayed ZSET key
// Default: zset:notifications:delayed
func DelayedZSetKey() string {
	return PrefixDelayed
}
