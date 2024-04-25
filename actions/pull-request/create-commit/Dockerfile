FROM alpine

# ENV GPG_TTY /dev/console
RUN apk add --no-cache \
    bash \
    git \
    gnupg

COPY entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
