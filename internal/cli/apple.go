package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
)

// applePollInterval is the time between two checks while waiting for App
// Store Connect processing.
const applePollInterval = 10 * time.Second

func newAppleCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apple",
		Short: "Manage apps in App Store Connect",
	}
	makeGroup(cmd)
	cmd.PersistentFlags().String("app", "", "bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)")
	cmd.PersistentFlags().String("platform", "ios", "platform of versions, builds and screenshots: ios, macos, tvos or visionos")
	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		p, _ := c.Flags().GetString("platform")
		if _, ok := apple.APIPlatform(p); !ok {
			return clierr.Usagef("invalid --platform %q (use ios, macos, tvos or visionos)", p)
		}
		return nil
	}
	cmd.AddCommand(newAppleAuthCmd(d), newAppleAppCmd(d), newAppleBuildCmd(d), newAppleTestflightCmd(d), newAppleVersionCmd(d))
	return cmd
}

// appleService returns the Apple service and the app given by --app or
// APPLE_APPSTORE_BUNDLE_ID. Only valid in command bodies.
func appleService(cmd *cobra.Command) (*apple.Service, apple.App, error) {
	bundleID, _ := cmd.Flags().GetString("app")
	if bundleID == "" {
		v, _ := cmdDeps(cmd).Env(config.EnvAppleBundleID)
		bundleID = strings.TrimSpace(v)
	}
	if bundleID == "" {
		return nil, apple.App{}, clierr.Usagef("no app given: pass --app <bundle ID> or set %s", config.EnvAppleBundleID)
	}
	svc, err := appleServiceNoApp(cmd)
	if err != nil {
		return nil, apple.App{}, err
	}
	app, err := svc.ResolveApp(cmd.Context(), bundleID)
	if err != nil {
		return nil, apple.App{}, err
	}
	return svc, app, nil
}

// appleServiceNoApp returns the Apple service for commands without an app.
// Only valid in command bodies.
func appleServiceNoApp(cmd *cobra.Command) (*apple.Service, error) {
	creds, err := appleCredentials(cmd)
	if err != nil {
		return nil, err
	}
	return newAppleService(cmd, creds)
}

func appleCredentials(cmd *cobra.Command) (config.AppleCredentials, error) {
	d := cmdDeps(cmd)
	return config.LoadAppleCredentials(d.Env, d.ReadFile)
}

func newAppleService(cmd *cobra.Command, creds config.AppleCredentials) (*apple.Service, error) {
	d := cmdDeps(cmd)
	baseURL := asc.DefaultBaseURL
	if v, ok := d.Env(config.EnvInternalAppleAPIURL); ok && v != "" {
		baseURL = v
	}
	c, err := ascclient.New(creds, httpClient(cmd), baseURL, d.Now)
	if err != nil {
		return nil, err
	}
	return apple.NewService(c, printer(cmd), apple.Poll{Interval: applePollInterval, Sleep: d.Sleep}, d.Now), nil
}

// appleAPIError turns an App Store Connect error response anywhere in err's
// chain into a clierr.Error, keeping the context added by wrappers
// ("attach build 42: ..."); other errors pass unchanged.
func appleAPIError(err error) error {
	var re *asc.ResponseError
	if !errors.As(err, &re) {
		return err
	}
	ce := ascclient.FromAPIError(re)
	if prefix, ok := strings.CutSuffix(err.Error(), re.Error()); ok {
		ce.Message = prefix + ce.Message
	}
	return ce
}
