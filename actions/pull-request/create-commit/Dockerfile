FROM alpine

# ENV GPG_TTY /dev/console
RUN apk add --no-cache \
    bash \
    git \
    gnupg \
    ca-certificates

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
