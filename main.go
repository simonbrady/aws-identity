// Command-line tool to manage STS credentials
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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
)

func init() {
	boolOpts := []struct {
		variable  *bool
		shortName string
		longName  string
		usage     string
	}{
		{&debug, "D", "debug", "Enable debugging output"},
		{&quiet, "q", "quiet", "Minimise output"},
	}
	for _, opt := range boolOpts {
		flag.BoolVar(opt.variable, opt.shortName, false, opt.usage)
		flag.BoolVar(opt.variable, opt.longName, false, opt.usage)
	}

	int64Opts := []struct {
		variable     *int64
		shortName    string
		longName     string
		defaultValue int64
		usage        string
	}{
		{&duration, "d", "duration", 3600, "Lifetime of temporary credentials in seconds"},
	}
	for _, opt := range int64Opts {
		flag.Int64Var(opt.variable, opt.shortName, opt.defaultValue, opt.usage)
		flag.Int64Var(opt.variable, opt.longName, opt.defaultValue, opt.usage)
	}

	stringOpts := []struct {
		variable  *string
		shortName string
		longName  string
		usage     string
	}{
		{&account, "a", "account", "Target account, default derived from caller identity"},
		{&mfaSerial, "m", "mfa-serial", "ARN of MFA device, default derived from caller identity"},
		{&mfaToken, "t", "mfa-token", "MFA token code"},
		{&role, "r", "role", "Target role name"},
		{&sessionName, "n", "session-name", "Session name, default derived from caller identity"},
	}
	for _, opt := range stringOpts {
		flag.StringVar(opt.variable, opt.shortName, "", opt.usage)
		flag.StringVar(opt.variable, opt.longName, "", opt.usage)
	}

	flag.Parse()
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
	if !quiet {
		fmt.Printf("Spawning subshell for %s\n", principal)
	}
	cmd := exec.Command("bash", "-i")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+*creds.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY="+*creds.SecretAccessKey,
		"AWS_SESSION_TOKEN="+*creds.SessionToken)
	cmd.Run()
}

func main() {
	log.SetPrefix("aws-identity: ")
	log.SetFlags(0)
	client := newClient()

	// Infer any missing options from the current user's identity
	identity, err := client.GetCallerIdentity(new(sts.GetCallerIdentityInput))
	if err != nil {
		log.Fatal(err)
	}
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
		if err != nil {
			log.Fatal(err)
		}
		spawnSubShell("role "+roleArn, creds)
	case mfaToken != "":
		creds, err := getMFACreds(client)
		if err != nil {
			log.Fatal(err)
		}
		spawnSubShell("user "+*identity.Arn, creds)
	default:
		fmt.Println(*identity.Arn)
	}
}
