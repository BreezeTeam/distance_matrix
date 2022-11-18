// Package sdk
// @Author Euraxluo  17:49:00
package sdk

import (
	"github.com/pkg/errors"
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		code int
		err  string
	}
	tests := []struct {
		name string
		args args
		want Error
	}{
		{name: "1", args: args{code: 10001, err: "test1"}, want: Error{code: 10001}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := Define(tt.args.code, tt.args.err); !Equal(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObtain(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want Error
	}{
		{name: "1", s: "test2", want: Error{code: 500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Obtain(tt.s)
			t.Logf("New() = %v, want %v", got, tt.want)
		})
	}
}

func TestCause(t *testing.T) {
	tests := []struct {
		name string
		s    error
		want Error
	}{
		{name: "1", s: errors.New("test3"), want: Error{code: 500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cause(tt.s); !Equal(got, tt.want) {
				t.Errorf("New() = %v, want %v", got.Message(), tt.want)
			}
		})
	}
}
