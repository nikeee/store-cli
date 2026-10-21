package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestHTTPClientUsesVerboseStderrAndSleep(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	for _, verbose := range []bool{true, false} {
		n.Store(0)
		var out, errb bytes.Buffer
		var slept []time.Duration
		d := DefaultDeps()
		d.Stdout, d.Stderr = &out, &errb
		d.Sleep = func(_ context.Context, dur time.Duration) error {
			slept = append(slept, dur)
			return nil
		}
		root := NewRootCmd(d)
		var status int
		root.AddCommand(&cobra.Command{
			Use: "probe",
			RunE: func(cmd *cobra.Command, _ []string) error {
				resp, err := httpClient(cmd).Get(srv.URL + "/v1/x?access_token=tok")
				if err != nil {
					return err
				}
				_ = resp.Body.Close()
				status = resp.StatusCode
				return nil
			},
		})
		args := []string{"probe"}
		if verbose {
			args = append(args, "--verbose")
		}
		root.SetArgs(args)
		if err := root.ExecuteContext(context.Background()); err != nil {
			t.Fatal(err)
		}
		if status != 200 || len(slept) != 1 {
			t.Fatalf("verbose=%v status=%d slept=%v", verbose, status, slept)
		}
		log := errb.String()
		if verbose {
			if !strings.Contains(log, "--> GET "+srv.URL+"/v1/x?access_token=***") || !strings.Contains(log, "retry 2/5") || strings.Contains(log, "tok\n") {
				t.Errorf("stderr=%q", log)
			}
		} else if log != "" {
			t.Errorf("stderr=%q", log)
		}
		if out.Len() != 0 {
			t.Errorf("stdout=%q", out.String())
		}
	}
}
