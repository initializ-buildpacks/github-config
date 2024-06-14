FROM alpine

RUN apk add --no-cache bash

COPY entrypoint /entrypoint

# Ensure the entrypoint script has execute permissions
RUN chmod +x /entrypoint

ENTRYPOINT ["/entrypoint"]