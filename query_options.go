package gocb

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QueryScanConsistency indicates the level of data consistency desired for a query.
type QueryScanConsistency uint

const (
	// QueryScanConsistencyNotBounded indicates no data consistency is required.
	QueryScanConsistencyNotBounded = QueryScanConsistency(1)
	// QueryScanConsistencyRequestPlus indicates that request-level data consistency is required.
	QueryScanConsistencyRequestPlus = QueryScanConsistency(2)
)

// QueryOptions represents the options available when executing a query.
type QueryOptions struct {
	ScanConsistency      QueryScanConsistency
	ConsistentWith       *MutationState
	Profile              QueryProfileMode
	ScanCap              uint32
	PipelineBatch        uint32
	PipelineCap          uint32
	ScanWait             time.Duration
	Readonly             bool
	MaxParallelism       uint32
	ClientContextID      string
	PositionalParameters []interface{}
	NamedParameters      map[string]interface{}
	Metrics              bool
	Raw                  map[string]interface{}

	Adhoc         bool
	Timeout       time.Duration
	RetryStrategy RetryStrategy

	parentSpan requestSpanContext
}

func (opts *QueryOptions) toMap() (map[string]interface{}, error) {
	execOpts := make(map[string]interface{})

	if opts.ScanConsistency != 0 && opts.ConsistentWith != nil {
		return nil, makeInvalidArgumentsError("ScanConsistency and ConsistentWith must be used exclusively")
	}

	if opts.ScanConsistency != 0 {
		if opts.ScanConsistency == QueryScanConsistencyNotBounded {
			execOpts["scan_consistency"] = "not_bounded"
		} else if opts.ScanConsistency == QueryScanConsistencyRequestPlus {
			execOpts["scan_consistency"] = "request_plus"
		} else {
			return nil, makeInvalidArgumentsError("Unexpected consistency option")
		}
	}

	if opts.ConsistentWith != nil {
		execOpts["scan_consistency"] = "at_plus"
		execOpts["scan_vectors"] = opts.ConsistentWith
	}

	if opts.Profile != "" {
		execOpts["profile"] = opts.Profile
	}

	if opts.Readonly {
		execOpts["readonly"] = opts.Readonly
	}

	if opts.PositionalParameters != nil && opts.NamedParameters != nil {
		return nil, makeInvalidArgumentsError("Positional and named parameters must be used exclusively")
	}

	if opts.PositionalParameters != nil {
		execOpts["args"] = opts.PositionalParameters
	}

	if opts.NamedParameters != nil {
		for key, value := range opts.NamedParameters {
			if !strings.HasPrefix(key, "$") {
				key = "$" + key
			}
			execOpts[key] = value
		}
	}

	if opts.ScanCap != 0 {
		execOpts["scan_cap"] = strconv.FormatUint(uint64(opts.ScanCap), 10)
	}

	if opts.PipelineBatch != 0 {
		execOpts["pipeline_batch"] = strconv.FormatUint(uint64(opts.PipelineBatch), 10)
	}

	if opts.PipelineCap != 0 {
		execOpts["pipeline_cap"] = strconv.FormatUint(uint64(opts.PipelineCap), 10)
	}

	if opts.ScanWait > 0 {
		execOpts["scan_wait"] = opts.ScanWait.String()
	}

	if opts.Raw != nil {
		for k, v := range opts.Raw {
			execOpts[k] = v
		}
	}

	if opts.MaxParallelism > 0 {
		execOpts["max_parallelism"] = strconv.FormatUint(uint64(opts.MaxParallelism), 10)
	}

	if !opts.Metrics {
		execOpts["metrics"] = false
	}

	if opts.ClientContextID == "" {
		execOpts["client_context_id"] = uuid.New()
	} else {
		execOpts["client_context_id"] = opts.ClientContextID
	}

	return execOpts, nil
}
