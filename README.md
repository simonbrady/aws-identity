# aws-identity

A simple command-line tool to manage
[temporary STS credentials](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp.html), written using the
[AWS SDK for Go](https://aws.amazon.com/sdk-for-go/). When run without options it will use
[GetCallerIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetCallerIdentity.html)
to display the current user identity. It can also get temporary credentials and inject them into a new shell process
as [CLI environment variables](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html).

To see a full list of options, use the `-h` or `--help` command-line option.

## Assume a named role in the same account

Get the account for the current user identity (which could be set through a
[named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html))
then use [AssumeRole](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html)
to assume a named role in that account. Takes an optional
[MFA token code](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_mfa.html) if the trust condition for
assuming the role requires MFA.

```
aws-identity -r <role-name> [-t token-code]
```

e.g.

```
$ aws-identity -r admin -t 123456
Spawning subshell for role arn:aws:iam::111122223333:role/admin
```

### Assume a named cross-account role

As above but takes the target account number to assume the role in.

```
aws-identity -a <account> -r role-name [-t token-code]
```

e.g.

```
$ aws-identity -a 444455556666 -r admin -t 234567
Spawning subshell for role arn:aws:iam::444455556666:role/admin
```

### Authenticate the current user with MFA

Rather than assuming a new role identity, use
[GetSessionToken](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetSessionToken.html)
to generate temporary credentials for the current identity but with MFA.

```
aws-identity -t <token-code>
```

e.g.

```
$ aws-identity -t 345678
Spawning subshell for user arn:aws:iam::111122223333:user/jrh
```

This is useful for tools like the [Terraform AWS provider](https://www.terraform.io/docs/providers/aws/)
that can assume roles but don't prompt for an MFA token.
