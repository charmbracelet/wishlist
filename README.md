# wishlist

Wishlist is a directory for SSH Apps, and a SSH app itself.

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
  -o 'ForwardAgent=yes' \ # forwards the agent
  -o 'UserKnownHostsFile=/dev/null' \ # do not add to ~/.ssh/known_hosts, optional
  -p 2222 \ # port
  foo.bar \ # host
  -t list # optional, app name
```

## Running

Wishlist will readn and store all its information in a `.wishlist` folder in the current working directory:

- the server keys
- the client keys
- known hosts
- config files

The config files are tried in the following order:

- the `-config` flag in either YAML or SSH config formats - [[example YAML](/_example/config.yaml)|[example SSH config](/example/config.ssh_config)]
- `.wishlist/config.yaml` - [[example](/_example/config.yaml)]
- `.wishlist/config.yml` - [[example](/_example/config.yaml)]
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
