FROM alpine

RUN apk add --no-cache \
    bash \
    git \
    github-cli \
    jq \
    && rm -rf /var/cache/apk/*

COPY entrypoint /entrypoint

# Ensure the entrypoint script has execute permissions
RUN chmod +x /entrypoint

ENTRYPOINT ["/entrypoint"]
