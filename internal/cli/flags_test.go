package cli_test

import (
	"io/fs"
	"testing"

	"github.com/nikeee/store-cli/internal/cli"
	"github.com/nikeee/store-cli/internal/clierr"
)

func TestTextFlag(t *testing.T) {
	read := func(name string) ([]byte, error) {
		if name == "notes.txt" {
			return []byte("line1\nline2\n\n"), nil
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	tests := []struct {
		in      string
		want    string
		wantErr string
	}{
		{in: "hello", want: "hello"},
		{in: "@notes.txt", want: "line1\nline2"},
		{in: "@@x", want: "@x"},
		{in: "@missing.txt", wantErr: "--description: open missing.txt: file does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			f := cli.NewTextFlag("description", read)
			err := f.Set(tt.in)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v", err)
				}
				if clierr.ExitCode(err) != 2 {
					t.Fatalf("exit = %d, want 2", clierr.ExitCode(err))
				}
				return
			}
			if err != nil || f.Value != tt.want || !f.Changed {
				t.Fatalf("value=%q err=%v", f.Value, err)
			}
		})
	}
}
