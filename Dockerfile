FROM scratch
COPY wishlist /usr/local/bin/wishlist
ENTRYPOINT [ "wishlist" ]
