# Dockerfile for btcx
# This is used by GoReleaser to build multi-arch images

FROM alpine:3.20

# Install ca-certificates for HTTPS and git for cloning repos
RUN apk add --no-cache ca-certificates git

# Copy the binary from GoReleaser
COPY btcx /usr/local/bin/btcx

# Create a non-root user
RUN adduser -D -h /home/btcx btcx
USER btcx
WORKDIR /home/btcx

# Set up config directory
RUN mkdir -p /home/btcx/.config/btcx /home/btcx/.cache/btcx /home/btcx/.local/share/btcx

ENTRYPOINT ["btcx"]
CMD ["--help"]
