---
name: Feature request
about: Suggest new functionality for this project
title: ''
labels: 'feature'
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

## Describe the problem you'd like to have solved

> A clear and concise description of what the problem is. Ex. I'm always frustrated when [...]

## Describe the ideal solution

> A clear and concise description of what you want to happen.

## Alternatives and current workarounds

> A clear and concise description of any alternatives you've considered or any workarounds that are currently in place.

### Environment

> Please provide the following:

#### For ubuntu users, please run and copy the following

1. `ubuntu-bug libpam-aad --save=/tmp/report`
1. Copy paste below `/tmp/report` content:

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