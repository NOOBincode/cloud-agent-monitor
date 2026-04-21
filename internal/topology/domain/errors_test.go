package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopologyError(t *testing.T) {
	tests := []struct {
		name       string
		op         string
		err        error
		msg        string
		wantErrStr string
	}{
		{
			name:       "error with message",
			op:         "discover_nodes",
			err:        ErrNodeNotFound,
			msg:        "service not found",
			wantErrStr: "topology discover_nodes: service not found: topology node not found",
		},
		{
			name:       "error without message",
			op:         "analyze_impact",
			err:        ErrGraphNotReady,
			msg:        "",
			wantErrStr: "topology analyze_impact: topology graph not ready",
		},
		{
			name:       "error with wrapped error",
			op:         "refresh_topology",
			err:        NewTopologyError("inner_op", ErrDiscoveryFailed, "inner message"),
			msg:        "outer message",
			wantErrStr: "topology refresh_topology: outer message: topology inner_op: inner message: topology discovery failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topoErr := NewTopologyError(tt.op, tt.err, tt.msg)
			assert.Equal(t, tt.wantErrStr, topoErr.Error())
		})
	}
}

func TestTopologyError_Unwrap(t *testing.T) {
	innerErr := ErrNodeNotFound
	topoErr := NewTopologyError("test_op", innerErr, "test message")

	unwrapped := topoErr.Unwrap()
	assert.Equal(t, innerErr, unwrapped)
}

func TestTopologyError_ErrorsIs(t *testing.T) {
	topoErr := NewTopologyError("test_op", ErrNodeNotFound, "test message")

	assert.True(t, errors.Is(topoErr, ErrNodeNotFound))
	assert.False(t, errors.Is(topoErr, ErrEdgeNotFound))
}

func TestTopologyError_ErrorsAs(t *testing.T) {
	innerTopoErr := NewTopologyError("inner_op", ErrDiscoveryFailed, "inner message")
	outerTopoErr := NewTopologyError("outer_op", innerTopoErr, "outer message")

	var target *TopologyError
	assert.True(t, errors.As(outerTopoErr, &target))
	assert.Equal(t, "outer_op", target.Op)
}

func TestIsNodeNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error match",
			err:  ErrNodeNotFound,
			want: true,
		},
		{
			name: "wrapped error match",
			err:  NewTopologyError("test", ErrNodeNotFound, "wrapped"),
			want: true,
		},
		{
			name: "different error",
			err:  ErrEdgeNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNodeNotFound(tt.err))
		})
	}
}

func TestIsEdgeNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error match",
			err:  ErrEdgeNotFound,
			want: true,
		},
		{
			name: "wrapped error match",
			err:  NewTopologyError("test", ErrEdgeNotFound, "wrapped"),
			want: true,
		},
		{
			name: "different error",
			err:  ErrNodeNotFound,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsEdgeNotFound(tt.err))
		})
	}
}

func TestIsTopologyEmpty(t *testing.T) {
	assert.True(t, IsTopologyEmpty(ErrTopologyEmpty))
	assert.True(t, IsTopologyEmpty(NewTopologyError("test", ErrTopologyEmpty, "")))
	assert.False(t, IsTopologyEmpty(ErrNodeNotFound))
}

func TestIsDiscoveryFailed(t *testing.T) {
	assert.True(t, IsDiscoveryFailed(ErrDiscoveryFailed))
	assert.True(t, IsDiscoveryFailed(NewTopologyError("test", ErrDiscoveryFailed, "")))
	assert.False(t, IsDiscoveryFailed(ErrNodeNotFound))
}

func TestIsCacheMiss(t *testing.T) {
	assert.True(t, IsCacheMiss(ErrCacheMiss))
	assert.True(t, IsCacheMiss(NewTopologyError("test", ErrCacheMiss, "")))
	assert.False(t, IsCacheMiss(ErrNodeNotFound))
}

func TestAllErrorTypes(t *testing.T) {
	allErrors := []error{
		ErrNodeNotFound,
		ErrEdgeNotFound,
		ErrTopologyEmpty,
		ErrDiscoveryFailed,
		ErrInvalidQuery,
		ErrInvalidNodeID,
		ErrInvalidEdgeID,
		ErrGraphNotReady,
		ErrCacheMiss,
		ErrCircularDependency,
		ErrPathNotFound,
	}

	for i, err := range allErrors {
		t.Run(err.Error(), func(t *testing.T) {
			assert.NotNil(t, err, "Error at index %d should not be nil", i)
			assert.NotEmpty(t, err.Error(), "Error message should not be empty")
		})
	}
}
