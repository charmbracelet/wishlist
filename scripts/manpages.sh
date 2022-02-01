#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/wishlist man | gzip -c >manpages/wishlist.1.gz
