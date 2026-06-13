package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDispatch_Routing(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantCode  int
		wantOnOut string // substring that MUST appear on stdout ("" = stdout must be empty)
		wantOnErr string // substring that MUST appear on stderr ("" = stderr must be empty)
	}{
		{
			name:      "no_args_prints_usage_to_stdout",
			args:      nil,
			wantCode:  0,
			wantOnOut: "Usage: phantom",
			wantOnErr: "",
		},
		{
			name:      "empty_string_subcommand_prints_usage_to_stdout",
			args:      []string{""},
			wantCode:  0,
			wantOnOut: "Usage: phantom",
			wantOnErr: "",
		},
		{
			name:      "help_word_prints_usage_to_stdout",
			args:      []string{"help"},
			wantCode:  0,
			wantOnOut: "Usage: phantom",
			wantOnErr: "",
		},
		{
			name:      "dash_h_prints_usage_to_stdout",
			args:      []string{"-h"},
			wantCode:  0,
			wantOnOut: "Usage: phantom",
			wantOnErr: "",
		},
		{
			name:      "dash_dash_help_prints_usage_to_stdout",
			args:      []string{"--help"},
			wantCode:  0,
			wantOnOut: "Usage: phantom",
			wantOnErr: "",
		},
		{
			name:      "unknown_subcommand_prints_usage_to_stderr_code_2",
			args:      []string{"frobnicate"},
			wantCode:  2,
			wantOnOut: "",
			wantOnErr: "unknown subcommand",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			got := dispatch(tc.args, &out, &errBuf)
			if got != tc.wantCode {
				t.Fatalf("dispatch(%v) = %d, want %d", tc.args, got, tc.wantCode)
			}
			if tc.wantOnOut == "" {
				if out.Len() != 0 {
					t.Fatalf("stdout = %q, want empty", out.String())
				}
			} else if !strings.Contains(out.String(), tc.wantOnOut) {
				t.Fatalf("stdout = %q, want substring %q", out.String(), tc.wantOnOut)
			}
			if tc.wantOnErr == "" {
				if errBuf.Len() != 0 {
					t.Fatalf("stderr = %q, want empty", errBuf.String())
				}
			} else if !strings.Contains(errBuf.String(), tc.wantOnErr) {
				t.Fatalf("stderr = %q, want substring %q", errBuf.String(), tc.wantOnErr)
			}
		})
	}
}
