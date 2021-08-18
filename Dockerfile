# Copyright 2021 Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.
#
# (MIT License)

# Dockerfile for the console-data service

# Build will be where we build the go binary
FROM arti.dev.cray.com/baseos-docker-master-local/golang:1.14-alpine3.12 as build
RUN set -eux \
    && apk update \
    && apk add build-base

# Configure go env - installed as package but not quite configured
ENV GOPATH=/usr/local/golib
RUN export GOPATH=$GOPATH

# Copy in all the necessary files
COPY console_data_svc/*.go $GOPATH/src/
COPY vendor/ $GOPATH/src/

# Build the image
RUN set -ex && go build -v -i -o /app/console_data_svc $GOPATH/src/*.go

### Final Stage ###
# Start with a fresh image so build tools are not included
FROM arti.dev.cray.com/baseos-docker-master-local/alpine:3.13.5 as base

RUN set -eux \
    && apk update \
    && apk add postgresql-client curl

# Copy in the needed files
COPY --from=build /app/console_data_svc /app/

RUN echo 'alias ll="ls -l"' > ~/.bashrc
RUN echo 'alias vi="vim"' >> ~/.bashrc

ENTRYPOINT ["/app/console_data_svc"]
