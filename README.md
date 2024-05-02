# Azure Active Directory Authentication for Ubuntu

[![Code quality](https://github.com/ubuntu/aad-auth/workflows/QA/badge.svg)](https://github.com/ubuntu/aad-auth/actions?query=workflow%3AQA)
[![Code coverage](https://codecov.io/gh/ubuntu/aad-auth/branch/master/graph/badge.svg)](https://codecov.io/gh/ubuntu/aad-auth)
[![Go Reference](https://pkg.go.dev/badge/github.com/ubuntu/aad-auth.svg)](https://pkg.go.dev/github.com/ubuntu/aad-auth)
[![License CLI](https://img.shields.io/badge/License-GPL3.0-blue.svg)](https://github.com/ubuntu/aad-auth/blob/main/COPYING)
[![License libraries](https://img.shields.io/badge/License-LGPL3.0-blue.svg)](https://github.com/ubuntu/aad-auth/blob/main/COPYING.LESSER)


> ***
> **_NOTICE:_**
> 
> The project aad-auth, which has been our first draft implementation limited to Azure, is being archived. We are grateful for your support, contributions and feedback. It has been invaluable to the project's development and informed our future direction.
> 
> Moving forward, we are excited to introduce a broader and more versatile project, authd, which replaces  aad-auth. This new initiative will extend the capabilities beyond Azure, supporting a wider range of platforms and services, such as OpenID Connect-based providers. 
>
> You can find the new project here: [authd GitHub Repository](https://github.com/ubuntu/authd)
>
> The aad-auth repository will be set to read-only and will remain available for archival purposes.
>
> Thank you once again for your dedication and contributions to the aad-auth project. We look forward to your continued support and enthusiasm at our new home on the authd project.
>
> The Ubuntu Enterprise Desktop Team
> ***

Azure AD User Authentication is only included in Ubuntu 23.04 and 23.10.

This project allows users to sign in an Ubuntu machine using Azure Active Directory credentials. It relies on [Microsoft Authentication Library](https://github.com/AzureAD/microsoft-authentication-library-for-go) to communicate with Microsoft service.

The following components are distributed:

 1. A PAM module for authentication.
 2. An NSS module to query the password, group and shadow databases.
 3. A command line tool to manage the local cache for offline authentication and the system's configuration.

Ubuntu AAD Authentication supports offline authentication. Once signed in online, you are entitled to offline login.

Offline login, meaning login in without Azure Active Directory being reachable, is allowed for a period of 90 days. Once this time has passed, the user won't be able to authenticate without having access to Azure Active Directory and reset the offline grace period.

This period can be modified in aad configuration file. See the related section below.

## Installation

### Package installation

AAD authentication module for Ubuntu is published as a debian package. To install it from the command line, open a terminal and run the following command:

```
sudo apt install libpam-aad libnss-aad
```

This command will install the required modules for PAM and NSS.

For NSS it'll update the file ```/etc/nsswitch.conf``` and add the service ```aad``` for the databases ```password```, ```group``` and ```shadow```.

For PAM it'll update the file ```/etc/pam.d/common-auth``` and add the following line after pam_unix and pam_sss if it is configured:

```
auth [success=1 default=ignore] pam_aad.so
```

### Automatic home directory creation

In order to get a home directory when network users login, ```pam_mkhomedir``` must be enabled. It will automatically create a home directory on first login. This step can be done by running the following command:

```
sudo pam-auth-update --enable mkhomedir
```

### Setting up the Azure Application

Ubuntu Azure Active Directory requires the creation of an application in Azure.
See [Use the portal to create an Azure AD application and service principal that can access resources](https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal) for instructions to create an application that can access resources and retrieve the tenant and application ID required for authentication.

### System configuration

Finally the system must be configured to point to the Azure tenant that hosts the directory. This is done with the file ```/etc/aad.conf```.

The [default template](https://github.com/ubuntu/aad-auth/blob/main/conf/aad.conf.template) distributed with the package details the possible settings.

```
### required values
## See https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal
## for more information on how to set up an Azure AD app.
# tenant_id = xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
# app_id = yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy

### optional values (defaults)
# offline_credentials_expiration = 90 ; duration in days a user can log in without online verification
                                      ; set to 0 to prevent old users from being cleaned and allow offline authentication for an undetermined amount of time
                                      ; set to a negative value to prevent offline authentication
# homedir = /home/%f ; home directory pattern for the user, the following mapping applies:
#                    ; %f - full username
#                    ; %U - UID
#                    ; %l - first char of username
#                    ; %u - username without domain
#                    ; %d - domain
# shell = /bin/bash ; default shell for the user

### overriding values for a specific domain, every value inside a section is optional
# [domain.com]
# tenant_id = aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
# app_id = bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
# offline_credentials_expiration = 30
# homedir = /home/domain.com/%u
# shell = /bin/zsh
```

## aad-cli - AAD Authentication management tool

```aad-cli``` is a command line tool which purpose is to help manage the configuration of the system and update the shell and home directory of a user.

See ```aad-cli --help``` for detailed usage.

## Troubleshooting

### Logging

Logging is done through the standard journal facility of the system which can be monitored and queried with ```journalctl```.

Debugging can be enabled:

* For PAM: by adding ```debug``` to the line containing the module ```pam_aad``` in ```/etc/pam.d/common-auth```.

```text
auth [success=1 default=ignore] pam_aad.so debug
```

* For NSS: by adding the line ```NSS_AAD_DEBUG=1``` to ```/etc/environment```. Then reboot the machine to make it effective to the entire system.

After the previous steps, you can try logging in again and check the logs with the commands:
Remember that the logs will be printed on the system logs, so you will need sudo privileges.

```bash
# You can use the -b flag to control how many boots the log will show (e.g. -b 0 will show the current boot only)
journalctl -b0 | grep pam_aad # this will show the PAM module logs

journalctl -b0 | grep nss_aad # this will show the NSS module logs

journalctl -b0 | grep _aad # this will show both logs
```

### Offline Cache

A local cache is used to allow offline authentication. This cache is located in ```/var/lib/aad/cache/```. It is entirely managed by the PAM and NSS modules. Users who didn't authenticate against AAD for a certain period of time are automatically deleted from the cache and won't be able to login even offline.
