# Start with the chantools base image
ARG VERSION
FROM guggero/chantools:${VERSION:-latest} AS golangbuilder

FROM tsl0922/ttyd:1.7.7-alpine@sha256:e17d5420fa78ea6271e32a06eec334adda6f54077e56e3969340fb47e604c24c AS final

# Define a root volume for data persistence.
VOLUME /chantools
WORKDIR /chantools

# We'll expect the lnd data directory to be mounted here.
VOLUME /lnd

# Copy the binaries and entrypoint from the builder image.
COPY --from=golangbuilder /bin/chantools /bin/
COPY ./docker/bash-wrapper.sh /bash-wrapper.sh

# Add bash.
RUN apk add --no-cache \
  bash \
  jq \
  ca-certificates \
