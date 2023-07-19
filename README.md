# Wishlist

<!--toc:start-->

- [Wishlist](#wishlist)
  - [Installation](#installation)
  - [Usage](#usage)
    - [CLI](#cli)
      - [Remote](#remote)
      - [Local](#local)
    - [Library](#library)
  - [Auth](#auth)
    - [Local mode](#local-mode)
    - [Server mode](#server-mode)
    - [Agent forwarding example](#agent-forwarding-example)
  - [Discovery](#discovery)
    - [Tailscale](#tailscale)
    - [Zeroconf/Avahi/mDNS/Bonjour](#zeroconfavahimdnsbonjour)
    - [SRV records](#srv-records)
    - [Hints](#hints)
  - [Running it](#running-it)
    - [Using the binary](#using-the-binary)
    - [Using Docker](#using-docker)
    - [Supported SSH Options](#supported-ssh-options)
  - [Acknowledgments](#acknowledgments)
  - [Feedback](#feedback)
  - [License](#license)

<!--toc:end-->

<p>
    <a href="https://github.com/charmbracelet/wishlist/releases"><img src="https://img.shields.io/github/release/charmbracelet/wishlist.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/wishlist?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/wishlist/actions"><img src="https://github.com/charmbracelet/wishlist/workflows/build/badge.svg" alt="Build Status"></a>
    <a href="https://nightly.link/charmbracelet/wishlist/workflows/nightly/main"><img src="https://shields.io/badge/-Nightly%20Builds-orange?logo=hackthebox&logoColor=fff&style=appveyor"/></a>
</p>

The SSH directory ✨

![screencast](https://user-images.githubusercontent.com/42545625/176265745-1749c5ae-bf8d-460f-865c-fae0e45cb448.gif)

With Wishlist you can have a single entrypoint for multiple SSH endpoints,
whether they are [Wish](https://github.com/charmbracelet/wish) apps or not.

As a server, it can be used to start multiple SSH apps within a single package
and list them over SSH. You can list apps provided elsewhere, too.

You can also use the `wishlist` CLI to list and connect to servers in your
`~/.ssh/config` or, a YAML config file.

## Installation

Use your fave package manager:

```bash
# macOS or Linux
brew install charmbracelet/tap/wishlist

# Arch Linux (btw)
yay -S wishlist-bin (or wishlist)

# Windows (with Scoop)
scoop install wishlist

# Nix
nix-env -iA nixpkgs.wishlist

# Debian/Ubuntu
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install wishlist

# Fedora/RHEL
echo '[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key' | sudo tee /etc/yum.repos.d/charm.repo
sudo yum install wishlist
```

Or download a pre-compiled binary or package from the [releases][releases] page.

Or just build it yourself (requires Go 1.18+):

```bash
git clone https://github.com/charmbracelet/wishlist.git
cd wishlist
go build ./cmd/wishlist/
```

[releases]: https://github.com/charmbracelet/wishlist/releases

## Usage

### CLI

#### Remote

If you just want a directory of existing servers, you can use the `wishlist` CLI
and a YAML config file. You can also just run it without any arguments to list
the servers in your `~/.ssh/config`. To start wishlist in server mode, you'll
need to use the `serve` subcommand:

```sh
wishlist serve
```

Check the [example config file](/_example/config.yaml) file as well as
`wishlist server --help` for details.

#### Local

If you want to explore your `~/.ssh/config`, you can run wishlist in local mode
with:

```sh
wishlist
```

Note that not all options are supported at this moment. Check the
[commented example config](/_example/config) for reference.

### Library

Wishlist is also available as a library, which allows you to start several apps
within the same process.
Check out the `_example` folder for a working example.

## Auth

### Local mode

When running in local mode, wishlist will first see if the current endpoint has
an `IdentityFile` specified.
If so, it'll try to use that.
If not, it'll see if there's a SSH Agent available, and use it.
Otherwise, it'll try the common key names in `~/.ssh`.

### Server mode

When running as a server, wishlist will first try to forward the current SSH
Agent.
If there's no agent, it'll create or use an existing ed25519 key present in
`.wishlist/client_ed25519`.
Password authentication is not supported at this moment.

### Agent forwarding example

```sh
eval (ssh-agent)
ssh-add -k # adds all your pubkeys
ssh-add -l # should list the added keys

ssh \
  -o 'ForwardAgent=yes' \             # forwards the agent
  -o 'UserKnownHostsFile=/dev/null' \ # do not add to ~/.ssh/known_hosts, optional
  -p 2222 \                           # port
  foo.bar \                           # host
  -t list                             # optional, app name
```

You can also add this to your `~/.ssh/config`, for instance:

```sshconfig
Host wishlist
	HostName foo.bar
	Port 2222
	ForwardAgent yes
	UserKnownHostsFile /dev/null
```

## Discovery

Wishlist can discover endpoints using Zeroconf, SRV Records, and [Tailscale][].

You can find a brief explanation and examples of all of them bellow.

Run `wishlist --help` to see all the options.

[Tailscale]: http://tailscale.com

### Tailscale

You can configure Wishlist to find all nodes in your **tailnet** and add them
as endpoints:

```bash
wishlist --tailscale.net=your_tailnet_name --tailscale.key=tskey-api-abc123...
```

You can use the [Hints](#Hints) to change the connection settings.

#### OAuth authentication

Tailscale API keys expire after 90 days. If you want something that doesn't
require you to intervene every couple of months, use OAuth Clients:

Create a client [here](https://login.tailscale.com/admin/settings/oauth).
The only scope needed is `devices:read`.

Instead of using `--tailscale.key` (or `$TAILSCALE_KEY`), set
`--tailscale.client.id` and `--tailscale.client.secret` (or
`$TAILSCALE_CLIENT_ID` and `$TAILSCALE_CLIENT_SECRET`, respectively).

### Zeroconf/Avahi/mDNS/Bonjour

You can enable this using the `--zeroconf.enabled` flag:

```bash
wishlist --zeroconf.enabled
```

Optionally, you can also specify a timeout with `--zeroconf.timeout` and, which
domain to look for with `--zeroconf.domain`.

Wishlist will look for `_ssh._tcp` services in the given domain.

You can use the [Hints](#Hints) to change the connection settings.

### SRV records

You can set Wishlist up to find nodes from DNS `SRV` records:

```bash
wishlist --srv.domain example.com
```

By default, Wishlist will set the name of the endpoint to the `SRV` target.
You can, however, customize that with a `TXT` record in the following format:

```txt
wishlist.name full.address:22=thename
```

So, in this case, a `SRV` record pointing to `full.address` on port `22` will
get the name `thename`.

### Hints

You can use the `hints` key in the YAML configuration file to hint settings into
discovered endpoints.

Check the [example configuration file](/_example/config.yaml) to learn
what options are available.

If you're using a SSH configuration file as the Wishlist configuration file,
it'll try to match the hosts with the rules in the given configuration.
Otherwise, the services will simply be added to the list.

The difference is that if a hints themselves won't show in the TUI, as of hosts
in the SSH configuration will.

## Running it

Wishlist will read and store all its information in a `.wishlist` folder in the
current working directory:

- the server keys
- the client keys
- known hosts
- config files

Config files may be provided in either YAML or SSH Config formats:

- [example YAML](/_example/config.yaml)
- [example SSH config](/_example/config)

The config files are tried in the following order:

- the `-config` flag in either YAML or SSH config formats
- `.wishlist/config.yaml`
- `.wishlist/config.yml`
- `.wishlist/config`
- `[[user config dir]]/wishlist/config.yaml`[^1]
- `[[user config dir]]/wishlist/config.yml`[^1]
- `[[user config dir]]/wishlist/config`[^1]
- `$HOME/.ssh/config`
- `/etc/ssh/ssh_config`

[^1]:
    i.e. `[[user config dir]]`: On Unix systems, it will be `$XDG_CONFIG_HOME`
    as specified by
    https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
    if non-empty, else `$HOME/.config`. On Darwin, it will be
    `$HOME/Library/Application Support`. On Windows, it will be `%AppData%`.
    On Plan 9, it will be `$home/lib`.

The first one that is loaded and parsed without errors will be used. This means
that if you have your common used hosts in your `~/.ssh/config`, you can simply
run `wishlist` and get it running right away. It also means that if you don't
want that, you can pass a path to `-config`, and it can be either a YAML, or a
SSH config file.

### Using the binary

```sh
wishlist
```

### Using Docker

```sh
mkdir .wishlist
$EDITOR .wishlist/config.yaml # either an YAML or a SSH config
docker run \
  -p 2222:22 \
  -v $PWD/.wishlist:/.wishlist \
  docker.io/charmcli/wishlist:latest
```

### Supported SSH Options

Not all SSH options are currently supported.
Here's a list of the ones that are:

- `User`
- `Hostname`
- `Port`
- `IdentityFiles`
- `ForwardAgent`
- `RequestTTY`
- `RemoteCommand`
- `SendEnv`
- `SetEnv`
- `ConnectTimeout`
- `Include`
- `PreferredAuthentications`
- `ProxyJump`

## Acknowledgments

The gif above shows a lot of [Maas Lalani’s](https://github.com/maaslalani) [confeTTY](https://github.com/maaslalani/confetty).

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
