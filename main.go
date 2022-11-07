// Command-line tool to manage STS credentials
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

// Command-line options
var (
	account     string
	debug       bool
	duration    int64
	mfaSerial   string
	mfaToken    string
	quiet       bool
	role        string
	sessionName string
	version     bool
)

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "aws-identity",
		Short: "Command-line tool to manage STS credentials",
		Long: `Spawn subshell after assuming role and/or authenticating with MFA.

If no flags are given, print the caller's current AWS identity instead.`,
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}
	rootCmd.Flags().StringVarP(&account, "account", "a", "", "Target account, default derived from caller identity")
	rootCmd.Flags().BoolVarP(&debug, "debug", "D", false, "Enable debugging output")
	rootCmd.Flags().Int64VarP(&duration, "duration", "d", 3600, "Lifetime of temporary credentials in seconds")
	rootCmd.Flags().StringVarP(&mfaSerial, "mfa-serial", "m", "", "ARN of MFA device, default derived from caller identity")
	rootCmd.Flags().StringVarP(&mfaToken, "mfa-token", "t", "", "MFA token code")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Minimise output")
	rootCmd.Flags().StringVarP(&role, "role", "r", "", "Target role name")
	rootCmd.Flags().StringVarP(&sessionName, "session-name", "n", "", "Session name, default derived from caller identity")
	rootCmd.Flags().BoolVarP(&version, "version", "v", false, "Show version and exit")
	check(rootCmd.Execute())
}

func run() {
	if version {
		fmt.Println("aws-identity v1.0.4")
		return
	}

	client := newClient()

	// Infer any missing options from the current user's identity
	identity, err := client.GetCallerIdentity(new(sts.GetCallerIdentityInput))
	check(err)
	arn := strings.Split(*identity.Arn, ":")
	username := strings.Split(arn[5], "/")[1]
	if account == "" {
		account = arn[4]
	}
	if mfaSerial == "" {
		mfaSerial = fmt.Sprintf("arn:aws:iam::%s:mfa/%s", arn[4], username)
	}
	if sessionName == "" {
		sessionName = username
	}

	// Choose operation to perform based on which options were supplied
	switch {
	case role != "":
		roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", account, role)
		creds, err := getRoleCreds(client, roleArn)
		check(err)
		spawnSubShell("role "+roleArn, creds)
	case mfaToken != "":
		creds, err := getMFACreds(client)
		check(err)
		spawnSubShell("user "+*identity.Arn, creds)
	default:
		fmt.Println(*identity.Arn)
	}
}

// newClient creates an STS client, with session-level debugging if requested on the command line.
func newClient() *sts.STS {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	config := aws.NewConfig()
	if debug {
		config.WithLogLevel(aws.LogDebugWithHTTPBody)
	}
	return sts.New(sess, config)
}

// getRoleCreds returns STS credentials from assuming the role given in roleArn.
func getRoleCreds(client *sts.STS, roleArn string) (*sts.Credentials, error) {
	input := new(sts.AssumeRoleInput).
		SetDurationSeconds(duration).
		SetRoleArn(roleArn).
		SetRoleSessionName(sessionName)
	if mfaToken != "" {
		input.SetSerialNumber(mfaSerial).SetTokenCode(mfaToken)
	}
	result, err := client.AssumeRole(input)
	if err != nil {
		return nil, err
	}
	return result.Credentials, nil
}

// getMFACreds returns STS credentials for the current user but in an MFA session.
func getMFACreds(client *sts.STS) (*sts.Credentials, error) {
	input := new(sts.GetSessionTokenInput).
		SetDurationSeconds(duration).
		SetSerialNumber(mfaSerial).
		SetTokenCode(mfaToken)
	result, err := client.GetSessionToken(input)
	if err != nil {
		return nil, err
	}
	return result.Credentials, nil
}

// spawnSubShell launches a shell for the named principal, injecting the given credentials.
func spawnSubShell(principal string, creds *sts.Credentials) {
	shell := "bash"
	for _, env := range os.Environ() {
		keyval := strings.SplitN(env, "=", 2)
		if keyval[0] == "SHELL" {
			shell = keyval[1]
			break
		}
	}
	if !quiet {
		fmt.Printf("Spawning %s for %s\n", shell, principal)
	}
	cmd := exec.Command(shell, "-i")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+*creds.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY="+*creds.SecretAccessKey,
		"AWS_SESSION_TOKEN="+*creds.SessionToken,
	)
	cmd.Run()
}
