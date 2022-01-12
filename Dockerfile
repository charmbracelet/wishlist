FROM gcr.io/distroless/static
COPY wishlist /usr/local/bin/wishlist
ENTRYPOINT [ "/usr/local/bin/wishlist" ]
