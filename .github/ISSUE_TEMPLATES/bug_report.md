---
name: Report an issue
about: Create a bug report about an existing issue.
title: ''
labels: ''
assignees: ''
---

## Introdutory notes

**Be careful with sensitive information and security vulnerabities**
> In order to report bugs that could contain sensitive information, use [Launchpad](https://bugs.launchpad.net/ubuntu/+source/aad-auth) instead.
> On Ubuntu machines, you can use `ubuntu-bug libpam-aad` to collect relevant information.

**Thank you for helping improve aad-auth!**
Please take a look at the template bellow and answer all relevant questions. Your additional work here
is greatly appreciated and will help us respond as soon as possible. For general support or usage questions, please refer to the [Ubuntu Discourse](https://discourse.ubuntu.com/c/desktop/) instead.
Finally, to avoid duplicates, please search the existing issues (even the closed ones) before submitting another one.

By submitting an Issue to this repository, you agree to the terms within the [Ubuntu Code of Conduct](https://ubuntu.com/community/code-of-conduct).

# Template

## Description

> Provide a clear and concise description of the issue, including what you expected to happen.

## Reproduction

> Detail the steps taken to reproduce this error, what was expected, and whether this issue can be reproduced consistently or if it is intermittent.
>
> Where applicable, please include:
>
> * Code sample to reproduce the issue
> * Log files (redact/remove sensitive information)
> * Application settings (redact/remove sensitive information)
> * Screenshots

### Environment

> Please provide the following:

#### For Ubuntu users, please follow these steps:

1. Run `ubuntu-bug libpam-aad --save=/tmp/report`
2. Remember to redact any sensitive information contained in the file.
3. Copy paste below `/tmp/report` content:

```raw
COPY REPORT CONTENT HERE.
```

#### Relevant information

Logging is done through the standard journal facility of the system which can be monitored and queried with ```journalctl```.

Debugging can be enabled:

* For PAM: by adding ```debug``` to the line containing the module ```pam_aad``` in ```/etc/pam.d/common-auth```.

```cfg
auth [success=1 default=ignore] pam_aad.so debug
```

* For NSS: by adding the line ```NSS_AAD_DEBUG=1``` to ```/etc/environment```. Then reboot the machine to make it effective to the entire system.

#### Additional context

> Add any other context about the problem here.
