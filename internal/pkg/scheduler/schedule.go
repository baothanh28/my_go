package scheduler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Schedule defines when a job should run.
type Schedule interface {
	// NextRun returns the next run time after the given time.
	NextRun(from time.Time) time.Time
	// String returns a human-readable representation of the schedule.
	String() string
	// Type returns the schedule type for serialization.
	Type() string
}

// ScheduleType identifies the type of schedule.
type ScheduleType string

const (
	ScheduleTypeCron     ScheduleType = "cron"
	ScheduleTypeInterval ScheduleType = "interval"
	ScheduleTypeOnce     ScheduleType = "once"
)

// CronSchedule represents a cron-based schedule.
type CronSchedule struct {
	Expression string
	schedule   cron.Schedule
}

// NewCronSchedule creates a new cron schedule from a cron expression.
func NewCronSchedule(expression string) (*CronSchedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	return &CronSchedule{
		Expression: expression,
		schedule:   schedule,
	}, nil
}

func (c *CronSchedule) NextRun(from time.Time) time.Time {
	return c.schedule.Next(from)
}

func (c *CronSchedule) String() string {
	return fmt.Sprintf("cron(%s)", c.Expression)
}

func (c *CronSchedule) Type() string {
	return string(ScheduleTypeCron)
}

func (c *CronSchedule) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":       c.Type(),
		"expression": c.Expression,
	})
}

// IntervalSchedule represents an interval-based schedule.
type IntervalSchedule struct {
	Interval time.Duration
}

// NewIntervalSchedule creates a new interval schedule.
func NewIntervalSchedule(interval time.Duration) *IntervalSchedule {
	return &IntervalSchedule{
		Interval: interval,
	}
}

func (i *IntervalSchedule) NextRun(from time.Time) time.Time {
	return from.Add(i.Interval)
}

func (i *IntervalSchedule) String() string {
	return fmt.Sprintf("every %s", i.Interval)
}

func (i *IntervalSchedule) Type() string {
	return string(ScheduleTypeInterval)
}

func (i *IntervalSchedule) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":     i.Type(),
		"interval": i.Interval.String(),
	})
}

// OnceSchedule represents a one-time schedule at a specific time.
type OnceSchedule struct {
	RunAt time.Time
	ran   bool
}

// NewOnceSchedule creates a new one-time schedule.
func NewOnceSchedule(runAt time.Time) *OnceSchedule {
	return &OnceSchedule{
		RunAt: runAt,
		ran:   false,
	}
}

func (o *OnceSchedule) NextRun(from time.Time) time.Time {
	if o.ran || o.RunAt.Before(from) {
		// Return zero time if already ran or past scheduled time
		return time.Time{}
	}
	return o.RunAt
}

func (o *OnceSchedule) String() string {
	return fmt.Sprintf("once at %s", o.RunAt.Format(time.RFC3339))
}

func (o *OnceSchedule) Type() string {
	return string(ScheduleTypeOnce)
}

func (o *OnceSchedule) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":   o.Type(),
		"run_at": o.RunAt.Format(time.RFC3339),
		"ran":    o.ran,
	})
}

// MarkRan marks the once schedule as executed.
func (o *OnceSchedule) MarkRan() {
	o.ran = true
}
