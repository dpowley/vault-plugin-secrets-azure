# Vault Plugin: Azure Secrets Backend [![Build Status](https://travis-ci.org/hashicorp/vault-plugin-secrets-azure.svg?branch=master)](https://travis-ci.org/hashicorp/vault-plugin-secrets-azure)

This is a standalone backend plugin for use with [Hashicorp Vault](https://www.github.com/hashicorp/vault).
This plugin generates revocable, time-limited Service Principals for Microsoft Azure.

**Please note**: We take Vault's security and our users' trust very seriously. If you believe you have found a security issue in Vault, _please responsibly disclose_ by contacting us at [security@hashicorp.com](mailto:security@hashicorp.com).

## Quick Links
- [Vault Website](https://www.vaultproject.io)
- [Azure Secrets Docs](https://www.vaultproject.io/docs/secrets/azure/index.html)
- [Vault Github Project](https://www.github.com/hashicorp/vault)

## Getting Started

This is a [Vault plugin](https://www.vaultproject.io/docs/internals/plugins.html)
and is meant to work with Vault. This guide assumes you have already installed Vault
and have a basic understanding of how Vault works.

Otherwise, first read this guide on how to [get started with Vault](https://www.vaultproject.io/intro/getting-started/install.html).

To learn specifically about how plugins work, see documentation on [Vault plugins](https://www.vaultproject.io/docs/internals/plugins.html).

## Usage

Please see [documentation for the plugin](https://www.vaultproject.io/docs/secrets/azure/index.html)
on the Vault website.

This plugin is currently built into Vault and by default is accessed
at `azure`. To enable this in a running Vault server:

```sh
$ vault secrets enable azure
Success! Enabled the azure secrets engine at: azure/
```


## Developing

If you wish to work on this plugin, you'll first need
[Go](https://www.golang.org) installed on your machine
(version 1.17+ is *required*).

For local dev first make sure Go is properly installed, including
setting up a [GOPATH](https://golang.org/doc/code.html#GOPATH).
Next, clone this repository into
`$GOPATH/src/github.com/hashicorp/vault-plugin-secrets-azure`.
You can then download any required build tools by bootstrapping your
environment:

```sh
$ make bootstrap
```

To compile a development version of this plugin, run `make` or `make dev`.
This will put the plugin binary in the `bin` and `$GOPATH/bin` folders. `dev`
mode will only generate the binary for your platform and is faster:

```sh
$ make
$ make dev
```

Put the plugin binary into a location of your choice. This directory
will be specified as the [`plugin_directory`](https://www.vaultproject.io/docs/configuration/index.html#plugin_directory)
in the Vault config used to start the server.

```json
...
plugin_directory = "path/to/plugin/directory"
...
```

Start a Vault server with this config file:
```sh
$ vault server -config=path/to/config.json ...
...
```

Once the server is started, register the plugin in the Vault server's [plugin catalog](https://www.vaultproject.io/docs/internals/plugins.html#plugin-catalog):

```sh
$ vault write sys/plugins/catalog/azure \
        sha256=<expected SHA256 Hex value of the plugin binary> \
        command="vault-plugin-secrets-azure"
...
Success! Data written to: sys/plugins/catalog/azure
```

Note you should generate a new sha256 checksum if you have made changes
to the plugin. Example using openssl:

```sh
openssl dgst -sha256 $GOPATH/vault-plugin-secrets-azure
...
SHA256(.../go/bin/vault-plugin-secrets-azure)= 896c13c0f5305daed381952a128322e02bc28a57d0c862a78cbc2ea66e8c6fa1
```

Enable the auth plugin backend using the secrets enable plugin command:

```sh
$ vault secrets enable -plugin-name='azure' plugin
...

Successfully enabled 'plugin' at 'azure'!
```

#### Tests

If you are developing this plugin and want to verify it is still
functioning (and you haven't broken anything else), we recommend
running the tests.

To run the tests, invoke `make test`:

```sh
$ make test
```

You can also specify a `TESTARGS` variable to filter tests like so:

```sh
$ make test TESTARGS='--run=TestConfig'
```

#### Acceptance Tests

Acceptance tests requires Azure access, and the following to be installed:
- [Docker](https://docs.docker.com/get-docker/)
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)
- [bats](https://bats-core.readthedocs.io/en/stable)

_You will need to be properly logged in to Azure with your subscription set. See
['Azure Provider: Authenticating using the Azure CLI'](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/guides/azure_cli)_
for more information.

```sh
$ make test-acceptance AZURE_TENANT_ID=<your_tenant_id>
```

Setting `WITH_DEV_PLUGIN=1` will first build the local plugin, and automatically register
it with the test Vault instance.

```sh
$ make test-acceptance AZURE_TENANT_ID=<your_tenant_id> WITH_DEV_PLUGIN=1
```

Running tests against Vault Enterprise requires a valid license, and specifying an enterprise docker image:

```sh
$ make test-acceptance AZURE_TENANT_ID=<your_tenant_id> \
  VAULT_LICENSE=........ \
  VAULT_IMAGE=hashicorp/vault-enterprise:latest
```

The `test-acceptance` make target also accepts the following environment based directives:

* `TESTS_FILTER`: a regex of Bats tests to run, useful when you only want to run a subset of the tests.
