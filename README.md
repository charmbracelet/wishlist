# wishlist

Wishlist is a directory for SSH Apps, and a SSH app itself.

It can be used to start multiple SSH apps within a single package, and provide a list of them over SSH as well.
You can also list apps provided elsewhere.

You can also use the `wishlist` CLI to just start a listing of external SSH apps based on a JSON config file.

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
