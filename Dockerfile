# Start with a Golang builder image.
FROM golang:1.24.6-alpine3.22@sha256:c8c5f95d64aa79b6547f3b626eb84b16a7ce18a139e3e9ca19a8c078b85ba80d AS golangbuilder

# Pass a tag, branch or a commit using build-arg. This allows a docker image to
# be built from a specified Git state.
ARG checkout="master"

# Install dependencies and install/build chantools.
RUN apk add --no-cache --update alpine-sdk make \
  && git clone https://github.com/lightninglabs/chantools /go/src/github.com/lightninglabs/chantools \
  && cd /go/src/github.com/lightninglabs/chantools \
  && git checkout $checkout \
  && make install

# Start a new, final image to reduce size.
FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 AS final

# Define a root volume for data persistence.
VOLUME /chantools
WORKDIR /chantools

# We'll use the default / directory as the home directory, since the /chantools
# folder will be overwritten if a volume is mounted there.
ENV HOME=/

# We'll expect the lnd data directory to be mounted here.
VOLUME /lnd

# Copy the binaries and entrypoint from the builder image.
COPY ./docker/docker-entrypoint.sh /bin/
COPY ./docker/bash-wrapper.sh /usr/local/bin/bash
COPY --from=golangbuilder /go/bin/chantools /bin/

# Make the wrapper executable.
RUN chmod 0777 /usr/local/bin/bash

# Add bash.
RUN apk add --no-cache \
  bash \
  jq \
  ca-certificates

# We'll want to just start a shell, but also give the user some info on how to
# use this image, which we do with a shell script.
ENTRYPOINT ["/bin/docker-entrypoint.sh"]
