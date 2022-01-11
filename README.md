Wishlist
==========

<p>
    <a href="https://github.com/charmbracelet/wishlist/releases"><img src="https://img.shields.io/github/release/charmbracelet/wishlist.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/wishlist?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/wishlist/actions"><img src="https://github.com/charmbracelet/wishlist/workflows/build/badge.svg" alt="Build Status"></a>
</p>

Wishlist is a directory for SSH Apps, and a SSH app itself.

![screencast](https://user-images.githubusercontent.com/245435/148814423-2a3b05b4-fb79-4ff9-a475-1942d6ffb363.gif)


It can be used to start multiple SSH apps within a single package, and provide a list of them over SSH as well.
You can also list apps provided elsewhere.

You can also use the `wishlist` CLI to just start a listing of external SSH apps based on a YAML config file.

## Usage

### CLI

If you just want a directory of existing apps, you can use the `wishlist` CLI and a YAML config file.
Check the `wishlist.example.yaml` file as well as `wishlist --help`

### Library

You can also use wishlist as a library, in which you can also start several apps within the same process.
Check the `_example` folder for a working example.

## Auth

* if ssh agent forwarding is available, it will be used
* otherwise, each session will create a new ed25519 key and use it, in which case your app will be to allow access to any public key
* password auth is not supported

### Example agent forwarding

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

## Running

Wishlist will read and store all its information in a `.wishlist` folder in the current working directory:

- the server keys
- the client keys
- known hosts
- config files

Config files may be provided in either YAML or SSH Config formats:

- [example YAML](/_example/config.yaml)
- [example SSH config](/_example/config.ssh_config)

The config files are tried in the following order:

- the `-config` flag in either YAML or SSH config formats
- `.wishlist/config.yaml`
- `.wishlist/config.yml`
- `$HOME/.ssh/config`
- `/etc/ssh/ssh_config`

The first one that is loaded and parsed without errors will be used.
This means that if you have your common used hosts in your `~/.ssh/config`, you can simply run `wishlist` and get it running right away.
It also means that if you don't want that, you can pass a path to `-config`, and it can be either a YAML or a SSH config file.

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

## Feedback

We'd love to hear your thoughts on this tutorial. Feel free to drop us a note!

* [Twitter](https://twitter.com/charmcli)
* [The Fediverse](https://mastodon.technology/@charm)

## License

[MIT](/LICENSE)

***

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge-unrounded.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source
